package docker

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/pv3dev/pv3/internal/project"
)

var (
	bold  = "\033[1m"
	dim   = "\033[2m"
	reset = "\033[0m"
	green = "\033[32m"
)

var dockerBin string

func init() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		bold, dim, reset, green = "", "", "", ""
	}
}

type RunConfig struct {
	Port    int
	NoNet   bool
	Image   string
	Verbose bool
	Dir     string // project directory (defaults to cwd if empty)
}

// resolveDir returns cfg.Dir if set, otherwise the current working directory.
func resolveDir(cfg RunConfig) (string, error) {
	if cfg.Dir != "" {
		return cfg.Dir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return dir, nil
}

// RunInstall installs project dependencies inside a Docker container.
func RunInstall(cfg RunConfig) error {
	if err := resolveRuntime(); err != nil {
		return err
	}

	cleanupOrphans()

	dir, err := resolveDir(cfg)
	if err != nil {
		return err
	}

	proj, err := project.ReadProject(dir)
	if err != nil {
		return err
	}

	if proj.InstallCmd == "" {
		return fmt.Errorf("no install command detected for %s project", proj.Runtime)
	}

	containerName := buildContainerName(dir)
	args := buildInstallArgs(cfg, dir, containerName, proj)

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "\n%s%s\n\n", dim, formatDockerCmd(dockerBin, args))
		fmt.Fprint(os.Stderr, reset)
	}

	fmt.Fprintf(os.Stderr, "\n%spv3%s %s%s install%s\n\n", bold, reset, dim, proj.PkgManager, reset)

	return executeInstall(containerName, args)
}

// Run executes the dev server inside a Docker container.
func Run(cfg RunConfig) error {
	if err := resolveRuntime(); err != nil {
		return err
	}

	cleanupOrphans()

	if err := checkPort(cfg.Port); err != nil {
		return err
	}

	dir, err := resolveDir(cfg)
	if err != nil {
		return err
	}

	proj, err := project.ReadProject(dir)
	if err != nil {
		return err
	}

	containerName := buildContainerName(dir)
	args := buildDockerArgs(cfg, dir, containerName, proj)

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "\n%s%s\n\n", dim, formatDockerCmd(dockerBin, args))
		fmt.Fprint(os.Stderr, reset)
	}

	fmt.Fprintf(os.Stderr, "\n%spv3%s %s%s run %s%s\n", bold, reset, dim, proj.PkgManager, proj.ScriptName, reset)
	fmt.Fprintf(os.Stderr, "%s    http://localhost:%d%s\n\n", dim, cfg.Port, reset)

	return execute(containerName, args)
}

// CloneRepo clones a git repository and returns the path to the cloned directory.
func CloneRepo(repoURL string) (string, error) {
	// Check git is available
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required but not found")
	}

	// Extract repo name from URL for the directory name
	repoName := extractRepoName(repoURL)
	if repoName == "" {
		return "", fmt.Errorf("could not determine repository name from URL: %s", repoURL)
	}

	// Clone into current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	destDir := filepath.Join(cwd, repoName)

	// If directory already exists, don't re-clone
	if _, err := os.Stat(destDir); err == nil {
		fmt.Fprintf(os.Stderr, "%s%s already exists, skipping clone%s\n", dim, repoName, reset)
		return destDir, nil
	}

	fmt.Fprintf(os.Stderr, "\n%spv3%s %scloning %s%s\n\n", bold, reset, dim, repoURL, reset)

	cmd := exec.Command(gitBin, "clone", "--depth=1", repoURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cloning repository: %w", err)
	}

	return destDir, nil
}

// extractRepoName gets the repo name from a git URL.
// Handles: https://github.com/user/repo.git, git@github.com:user/repo.git, user/repo
func extractRepoName(url string) string {
	// Strip trailing .git
	url = strings.TrimSuffix(url, ".git")
	// Strip trailing slash
	url = strings.TrimSuffix(url, "/")

	// Get the last path component
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return url[i+1:]
	}
	if i := strings.LastIndex(url, ":"); i >= 0 {
		return url[i+1:]
	}
	return url
}

// resolveRuntime finds docker or podman and verifies the daemon is running.
func resolveRuntime() error {
	for _, name := range []string{"docker", "podman"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}

		cmd := exec.Command(path, "info")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			if name == "podman" {
				return fmt.Errorf("Podman is installed but not running. Try: podman machine start")
			}
			return fmt.Errorf("Docker is installed but not running. Start Docker Desktop and try again.")
		}

		dockerBin = path
		return nil
	}

	return fmt.Errorf("Docker or Podman is required but not found. Install Docker: https://docs.docker.com/get-docker/")
}

// checkPort verifies the host port is available before launching Docker.
func checkPort(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("Port %d is already in use. Try: pv3 dev --port %d", port, port+1)
	}
	ln.Close()
	return nil
}

func buildContainerName(cwd string) string {
	base := filepath.Base(cwd)
	safe := sanitizeName(base)
	id := randomID(8)
	return fmt.Sprintf("pv3-dev-%s-%s", safe, id)
}

// sanitizeName lowercases and strips non-alphanumeric chars, replacing them with hyphens.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 30 {
		name = name[:30]
	}
	if name == "" {
		name = "project"
	}
	return name
}

func randomID(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func buildInstallArgs(cfg RunConfig, cwd, containerName string, proj *project.ProjectInfo) []string {
	args := []string{
		"run",
		"--rm",
		"-it",
		"--name", containerName,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", fmt.Sprintf("%s:/workspace:delegated", cwd),
		"-w", "/workspace",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges:true",
		"--cpus=4",
		"--memory=6g",
	}

	if cfg.NoNet {
		args = append(args, "--network=none")
	}

	image := cfg.Image
	if image == "" {
		image = proj.Image
	}
	args = append(args, image)
	args = append(args, "sh", "-c", proj.InstallCmd)

	return args
}

func buildDockerArgs(cfg RunConfig, cwd, containerName string, proj *project.ProjectInfo) []string {
	args := []string{
		"run",
		"--rm",
		"-it",
		"--name", containerName,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", fmt.Sprintf("%s:/workspace:delegated", cwd),
		"-w", "/workspace",
		"-p", fmt.Sprintf("%d:%d", cfg.Port, cfg.Port),
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges:true",
		"--cpus=4",
		"--memory=6g",
		"-e", "NODE_ENV=development",
	}

	if term := os.Getenv("TERM"); term != "" {
		args = append(args, "-e", fmt.Sprintf("TERM=%s", term))
	}

	envFile := filepath.Join(cwd, ".env")
	if _, err := os.Stat(envFile); err == nil {
		args = append(args, "--env-file", envFile)
	}

	if cfg.NoNet {
		args = append(args, "--network=none")
	}

	image := cfg.Image
	if image == "" {
		image = proj.Image
	}
	args = append(args, image)
	args = append(args, "sh", "-c", proj.RunCmd)

	return args
}

// executeInstall runs the install container and waits for it to finish.
func executeInstall(containerName string, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()

	select {
	case <-sigCh:
		signal.Stop(sigCh)
		fmt.Fprintf(os.Stderr, "\n%sStopping...%s\n", dim, reset)
		stopContainer(containerName)
		<-doneCh
		return fmt.Errorf("install cancelled")

	case err := <-doneCh:
		elapsed := time.Since(startTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n%sInstall failed.%s %s(%s)%s\n", bold, reset, dim, formatDuration(elapsed), reset)
			return err
		}
		fmt.Fprintf(os.Stderr, "\n%s%sDone.%s %s(%s)%s\n", green, bold, reset, dim, formatDuration(elapsed), reset)
		return nil
	}
}

// execute runs the Docker container with full TTY passthrough and signal forwarding.
func execute(containerName string, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()

	select {
	case <-sigCh:
		signal.Stop(sigCh)
		fmt.Fprintf(os.Stderr, "\n%sStopping...%s\n", dim, reset)
		stopContainer(containerName)
		err := <-doneCh
		elapsed := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "%s%sStopped.%s %s(%s)%s\n", green, bold, reset, dim, formatDuration(elapsed), reset)
		return err

	case err := <-doneCh:
		elapsed := time.Since(startTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n%sExited with error.%s %s(%s)%s\n", bold, reset, dim, formatDuration(elapsed), reset)
			return err
		}
		fmt.Fprintf(os.Stderr, "\n%s%sStopped.%s %s(%s)%s\n", green, bold, reset, dim, formatDuration(elapsed), reset)
		return nil
	}
}

// stopContainer tries docker stop with a 5s timeout, then docker kill.
func stopContainer(name string) {
	stop := exec.Command(dockerBin, "stop", "-t", "5", name)
	stop.Stdout = nil
	stop.Stderr = nil
	if err := stop.Run(); err != nil {
		kill := exec.Command(dockerBin, "kill", name)
		kill.Stdout = nil
		kill.Stderr = nil
		_ = kill.Run()
	}
}

// formatDockerCmd produces a readable multi-line docker command for --verbose output.
func formatDockerCmd(bin string, args []string) string {
	var lines []string
	lines = append(lines, bin)

	i := 0
	for i < len(args) {
		arg := args[i]

		// Once we hit a non-flag arg that's not "run", we've reached
		// the image + command portion. Print the rest as a single line.
		if !strings.HasPrefix(arg, "-") && arg != "run" {
			lines = append(lines, fmt.Sprintf("  %s", strings.Join(args[i:], " ")))
			break
		}

		// Flags with separate values (e.g. --name pv3-dev-app)
		if (strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-")) &&
			!strings.Contains(arg, "=") &&
			i+1 < len(args) &&
			!strings.HasPrefix(args[i+1], "-") {
			lines = append(lines, fmt.Sprintf("  %s %s", arg, args[i+1]))
			i += 2
		} else {
			lines = append(lines, fmt.Sprintf("  %s", arg))
			i++
		}
	}

	return strings.Join(lines, " \\\n")
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	secs := d.Seconds()
	if secs < 60 {
		return fmt.Sprintf("%.1fs", secs)
	}
	mins := int(secs) / 60
	remaining := int(secs) % 60
	return fmt.Sprintf("%dm%ds", mins, remaining)
}

// cleanupOrphans finds and kills any leftover pv3-dev containers from crashed sessions.
func cleanupOrphans() {
	out, err := exec.Command(dockerBin, "ps", "-q", "--filter", "name=pv3-dev-").Output()
	if err != nil || len(out) == 0 {
		return
	}

	ids := strings.Fields(strings.TrimSpace(string(out)))
	if len(ids) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "Cleaning up %d orphaned pv3 container(s)...\n", len(ids))
	killArgs := append([]string{"rm", "-f"}, ids...)
	cmd := exec.Command(dockerBin, killArgs...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
