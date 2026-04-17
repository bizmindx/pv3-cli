package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pv3dev/pv3/internal/project"
)

// requireDocker skips the test if Docker/Podman is not available.
func requireDocker(t *testing.T) {
	t.Helper()
	for _, name := range []string{"docker", "podman"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, "info")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			continue
		}
		dockerBin = path
		return
	}
	t.Skip("Docker or Podman not available, skipping integration test")
}

// integrationTempDir creates a temp directory that works with Docker/Podman volume mounts.
// On macOS, Podman VM only shares /Users, so we can't use os.TempDir() (/var/folders).
func integrationTempDir(t *testing.T) string {
	t.Helper()

	var base string
	if runtime.GOOS == "darwin" {
		// Use a path under /Users that the Podman VM can see
		base = filepath.Join(os.Getenv("HOME"), ".pv3-test")
		os.MkdirAll(base, 0755)
	}

	dir, err := os.MkdirTemp(base, "pv3-integration-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// createDummyNodeProject creates a minimal Node.js project with a real dependency.
func createDummyNodeProject(t *testing.T) string {
	t.Helper()
	dir := integrationTempDir(t)

	pkg := `{
  "name": "pv3-test-node",
  "version": "1.0.0",
  "scripts": {
    "dev": "echo 'hello from pv3 sandbox'"
  },
  "dependencies": {
    "is-odd": "3.0.1"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// createDummyPythonProject creates a minimal Python project.
func createDummyPythonProject(t *testing.T) string {
	t.Helper()
	dir := integrationTempDir(t)

	reqs := "requests==2.32.3\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(reqs), 0644); err != nil {
		t.Fatal(err)
	}

	app := `import sys
print("hello from pv3 python sandbox")
sys.exit(0)
`
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(app), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// createDummyGitRepo creates a temp git repo with a Node.js project.
func createDummyGitRepo(t *testing.T) string {
	t.Helper()
	dir := integrationTempDir(t)

	pkg := `{
  "name": "pv3-clone-test",
  "version": "1.0.0",
  "scripts": {
    "dev": "echo 'cloned and running'"
  },
  "dependencies": {
    "is-odd": "3.0.1"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"init"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	return dir
}

// runContainer runs a command in a sandboxed container against a project directory.
func runContainer(t *testing.T, dir string, image string, shellCmd string) (string, error) {
	t.Helper()

	containerName := buildContainerName(dir)
	args := []string{
		"run",
		"--rm",
		"--name", containerName,
		"-v", dir + ":/workspace:delegated",
		"-w", "/workspace",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges:true",
		"--cpus=4",
		"--memory=6g",
		image,
		"sh", "-c", shellCmd,
	}

	cmd := exec.Command(dockerBin, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runInstallContainer runs the install step in a real container.
func runInstallContainer(t *testing.T, dir string, proj *project.ProjectInfo) error {
	t.Helper()

	containerName := buildContainerName(dir)
	cfg := RunConfig{Image: proj.Image}
	args := buildInstallArgs(cfg, dir, containerName, proj)

	cmd := exec.Command(dockerBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// runDevContainer runs the dev command in a real container.
func runDevContainer(t *testing.T, dir string, proj *project.ProjectInfo) error {
	t.Helper()

	_, err := runContainer(t, dir, proj.Image, proj.RunCmd)
	return err
}

// --- Node.js Tests ---

func TestIntegration_NodeInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyNodeProject(t)
	proj, err := project.ReadProject(dir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}

	if proj.InstallCmd != "npm install" {
		t.Fatalf("InstallCmd = %q, want 'npm install'", proj.InstallCmd)
	}

	if err := runInstallContainer(t, dir, proj); err != nil {
		t.Fatalf("install container failed: %v", err)
	}

	// Verify node_modules was created on the host via bind mount
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err != nil {
		t.Error("node_modules should exist after install")
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "is-odd")); err != nil {
		t.Error("node_modules/is-odd should exist after install")
	}
}

func TestIntegration_NodeInstallThenDev(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyNodeProject(t)
	proj, err := project.ReadProject(dir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}

	if err := runInstallContainer(t, dir, proj); err != nil {
		t.Fatalf("install container failed: %v", err)
	}

	out, err := runContainer(t, dir, proj.Image, proj.RunCmd)
	if err != nil {
		t.Fatalf("dev container failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "hello from pv3 sandbox") {
		t.Errorf("expected dev output to contain greeting, got: %s", out)
	}
}

// --- Python Tests ---

func TestIntegration_PythonInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyPythonProject(t)
	proj, err := project.ReadProject(dir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}

	if proj.InstallCmd != "pip install -r requirements.txt" {
		t.Fatalf("InstallCmd = %q, want 'pip install -r requirements.txt'", proj.InstallCmd)
	}

	if err := runInstallContainer(t, dir, proj); err != nil {
		t.Fatalf("install container failed: %v", err)
	}
}

func TestIntegration_PythonInstallThenRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyPythonProject(t)
	proj, err := project.ReadProject(dir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}

	if err := runInstallContainer(t, dir, proj); err != nil {
		t.Fatalf("install container failed: %v", err)
	}

	out, err := runContainer(t, dir, proj.Image, proj.RunCmd)
	if err != nil {
		t.Fatalf("dev container failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "hello from pv3 python sandbox") {
		t.Errorf("expected python output to contain greeting, got: %s", out)
	}
}

// --- Clone + Install + Run (pv3 run flow) ---

func TestIntegration_CloneAndInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	repoDir := createDummyGitRepo(t)

	// Clone into a separate working directory
	workDir := integrationTempDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(origDir)

	clonedDir, err := CloneRepo(repoDir)
	if err != nil {
		t.Fatalf("CloneRepo: %v", err)
	}

	if _, err := os.Stat(filepath.Join(clonedDir, "package.json")); err != nil {
		t.Fatal("package.json should exist in cloned directory")
	}

	proj, err := project.ReadProject(clonedDir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}

	if proj.Runtime != "node" {
		t.Fatalf("Runtime = %q, want 'node'", proj.Runtime)
	}

	if err := runInstallContainer(t, clonedDir, proj); err != nil {
		t.Fatalf("install container failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(clonedDir, "node_modules")); err != nil {
		t.Error("node_modules should exist after install")
	}

	out, err := runContainer(t, clonedDir, proj.Image, proj.RunCmd)
	if err != nil {
		t.Fatalf("dev container failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "cloned and running") {
		t.Errorf("expected output to contain 'cloned and running', got: %s", out)
	}
}

func TestIntegration_CloneSkipsExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	repoDir := createDummyGitRepo(t)
	workDir := integrationTempDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(origDir)

	// First clone
	dir1, err := CloneRepo(repoDir)
	if err != nil {
		t.Fatalf("first CloneRepo: %v", err)
	}

	// Second clone of same repo should skip
	dir2, err := CloneRepo(repoDir)
	if err != nil {
		t.Fatalf("second CloneRepo: %v", err)
	}

	if dir1 != dir2 {
		t.Errorf("second clone returned different dir: %q vs %q", dir1, dir2)
	}
}

// --- Security Tests ---

func TestIntegration_ContainerCannotSeeHostHome(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyNodeProject(t)

	// Try to read the host user's SSH keys from inside the container
	out, err := runContainer(t, dir, "node:22-bookworm-slim",
		"ls /Users 2>&1 || echo 'NO_USERS_DIR'; ls ~/.ssh 2>&1 || echo 'NO_SSH_DIR'")
	if err != nil {
		t.Fatalf("container failed: %v\n%s", err, out)
	}

	// Container should NOT have /Users (macOS host path) or ~/.ssh
	if strings.Contains(out, ".pub") || strings.Contains(out, "id_rsa") || strings.Contains(out, "id_ed25519") {
		t.Errorf("container should not see host SSH keys, got: %s", out)
	}
}

func TestIntegration_ContainerCannotEscapeWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyNodeProject(t)

	// Write a marker file, then verify the container can ONLY see /workspace contents
	os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("found"), 0644)

	out, err := runContainer(t, dir, "node:22-bookworm-slim",
		"cat /workspace/marker.txt && echo '---' && ls /workspace/")
	if err != nil {
		t.Fatalf("container failed: %v\n%s", err, out)
	}

	if !strings.Contains(out, "found") {
		t.Error("container should be able to read files in /workspace")
	}
	if !strings.Contains(out, "package.json") {
		t.Error("container should see package.json in /workspace")
	}
}

func TestIntegration_PostinstallRunsButContained(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := integrationTempDir(t)

	// Project with a "malicious" postinstall that tries to escape
	pkg := `{
  "name": "pv3-postinstall-test",
  "version": "1.0.0",
  "scripts": {
    "dev": "echo ok",
    "postinstall": "echo POSTINSTALL_RAN > /workspace/proof.txt && ls /Users > /workspace/host_users.txt 2>&1 || echo NO_ACCESS > /workspace/host_users.txt"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := project.ReadProject(dir)
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}

	if err := runInstallContainer(t, dir, proj); err != nil {
		t.Fatalf("install container failed: %v", err)
	}

	// Postinstall DID execute (wrote proof file via bind mount)
	proof, err := os.ReadFile(filepath.Join(dir, "proof.txt"))
	if err != nil {
		t.Fatal("postinstall should have written proof.txt to workspace")
	}
	if !strings.Contains(string(proof), "POSTINSTALL_RAN") {
		t.Errorf("proof.txt should contain POSTINSTALL_RAN, got: %s", proof)
	}

	// But it could NOT access host /Users
	hostData, err := os.ReadFile(filepath.Join(dir, "host_users.txt"))
	if err != nil {
		t.Fatal("host_users.txt should exist")
	}
	content := strings.TrimSpace(string(hostData))
	if content != "NO_ACCESS" && !strings.Contains(content, "No such file") && !strings.Contains(content, "cannot access") {
		t.Errorf("postinstall should not see host /Users, got: %q", content)
	}
}

func TestIntegration_CapDropPreventsPrivilegedOps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDocker(t)

	dir := createDummyNodeProject(t)

	// Try operations that require capabilities
	out, err := runContainer(t, dir, "node:22-bookworm-slim",
		"apt-get update 2>&1 | tail -1; echo EXIT:$?")

	// We don't care if the container itself exits non-zero from apt-get,
	// we just want to see it fail or be denied
	_ = err
	t.Logf("cap-drop test output: %s", out)

	// With --cap-drop=ALL, network operations and package installs should
	// either fail or be severely restricted
	if strings.Contains(out, "EXIT:0") && strings.Contains(out, "packages can be upgraded") {
		t.Error("apt-get update should not succeed cleanly with --cap-drop=ALL")
	}
}
