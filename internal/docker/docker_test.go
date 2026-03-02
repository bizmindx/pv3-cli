package docker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pv3dev/pv3/internal/project"
)

// testdataDir returns the absolute path to the top-level testdata/ directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-app", "my-app"},
		{"My Cool App!", "my-cool-app"},
		{"node_modules", "node-modules"},
		{"   spaces   ", "spaces"},
		{"a-very-long-project-name-that-exceeds-thirty-characters", "a-very-long-project-name-that-"},
		{"!!!", "project"},
		{"", "project"},
		{"UPPERCASE", "uppercase"},
		{"dots.and.dots", "dots-and-dots"},
		{"already-clean", "already-clean"},
		{"123numeric", "123numeric"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildContainerName(t *testing.T) {
	name := buildContainerName("/Users/dev/my-project")

	if !strings.HasPrefix(name, "pv3-dev-my-project-") {
		t.Errorf("container name = %q, want prefix 'pv3-dev-my-project-'", name)
	}

	// Extract the random suffix (everything after "pv3-dev-my-project-")
	suffix := strings.TrimPrefix(name, "pv3-dev-my-project-")
	if len(suffix) != 8 {
		t.Errorf("random suffix %q length = %d, want 8", suffix, len(suffix))
	}

	for _, c := range suffix {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			t.Errorf("suffix contains invalid char %q", string(c))
		}
	}
}

func TestBuildContainerName_Uniqueness(t *testing.T) {
	names := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := buildContainerName("/Users/dev/project")
		if names[name] {
			t.Fatalf("duplicate container name on iteration %d: %s", i, name)
		}
		names[name] = true
	}
}

func TestRandomID_Length(t *testing.T) {
	for _, n := range []int{1, 4, 8, 16, 32} {
		id := randomID(n)
		if len(id) != n {
			t.Errorf("randomID(%d) length = %d", n, len(id))
		}
	}
}

func TestRandomID_Charset(t *testing.T) {
	// Generate a long ID to increase charset coverage
	id := randomID(1000)
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			t.Errorf("randomID contains invalid char: %c", c)
		}
	}
}

func TestBuildDockerArgs_CoreFlags(t *testing.T) {
	cfg := RunConfig{
		Port:  5173,
		NoNet: false,
		Image: "node:22-bookworm-slim",
	}
	proj := &project.ProjectInfo{
		ScriptName: "dev",
		ScriptCmd:  "vite",
		PkgManager: "npm",
		RunCmd:     "npm run dev",
	}

	args := buildDockerArgs(cfg, "/home/user/app", "pv3-dev-app-abc123", proj)

	// Verify required flags exist
	for _, want := range []string{"run", "--rm", "-it", "--cap-drop=ALL", "--cpus=4", "--memory=6g"} {
		assertContains(t, args, want)
	}

	// Verify flag-value pairs
	assertFlagValue(t, args, "--name", "pv3-dev-app-abc123")
	assertFlagValue(t, args, "-w", "/workspace")
	assertFlagValue(t, args, "-p", "5173:5173")
	assertFlagValue(t, args, "--security-opt", "no-new-privileges:true")
	assertFlagValue(t, args, "-v", "/home/user/app:/workspace:delegated")

	// Should NOT have --network=none when NoNet is false
	assertNotContains(t, args, "--network=none")

	// Image should appear before the shell command
	imageIdx := indexOf(args, "node:22-bookworm-slim")
	shIdx := indexOf(args, "sh")
	if imageIdx == -1 || shIdx == -1 || imageIdx >= shIdx {
		t.Error("image should appear before 'sh' in args")
	}

	// Final args must be: sh -c "npm run dev"
	if len(args) < 3 {
		t.Fatal("args too short")
	}
	tail := args[len(args)-3:]
	if tail[0] != "sh" || tail[1] != "-c" || tail[2] != "npm run dev" {
		t.Errorf("tail args = %v, want [sh -c npm run dev]", tail)
	}
}

func TestBuildDockerArgs_NoNet(t *testing.T) {
	cfg := RunConfig{Port: 5173, NoNet: true, Image: "node:22-bookworm-slim"}
	proj := &project.ProjectInfo{RunCmd: "npm run dev"}

	args := buildDockerArgs(cfg, "/home/user/app", "test", proj)
	assertContains(t, args, "--network=none")
}

func TestBuildDockerArgs_UserMatchesHost(t *testing.T) {
	cfg := RunConfig{Port: 5173, Image: "node:22-bookworm-slim"}
	proj := &project.ProjectInfo{RunCmd: "npm run dev"}

	args := buildDockerArgs(cfg, "/tmp/test", "test", proj)

	want := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	assertFlagValue(t, args, "--user", want)
}

func TestBuildDockerArgs_EnvFile(t *testing.T) {
	cfg := RunConfig{Port: 5173, Image: "node:22-bookworm-slim"}
	proj := &project.ProjectInfo{RunCmd: "npm run dev"}

	// Without .env — flag should be absent (use vite-react which has no .env)
	noEnvDir := filepath.Join(testdataDir(t), "javascript", "vite-react")
	args := buildDockerArgs(cfg, noEnvDir, "test", proj)
	for _, a := range args {
		if a == "--env-file" {
			t.Error("--env-file should not be present when .env doesn't exist")
		}
	}

	// With .env — flag should point to the file (use env-file fixture)
	envDir := filepath.Join(testdataDir(t), "javascript", "env-file")
	args = buildDockerArgs(cfg, envDir, "test", proj)
	assertFlagValue(t, args, "--env-file", filepath.Join(envDir, ".env"))
}

func TestBuildDockerArgs_TermPassthrough(t *testing.T) {
	cfg := RunConfig{Port: 5173, Image: "node:22-bookworm-slim"}
	proj := &project.ProjectInfo{RunCmd: "npm run dev"}

	original := os.Getenv("TERM")
	defer os.Setenv("TERM", original)

	// With TERM set
	os.Setenv("TERM", "xterm-256color")
	args := buildDockerArgs(cfg, "/tmp/test", "test", proj)
	found := false
	for i, a := range args {
		if a == "-e" && i+1 < len(args) && args[i+1] == "TERM=xterm-256color" {
			found = true
			break
		}
	}
	if !found {
		t.Error("TERM=xterm-256color should be in args when TERM is set")
	}

	// With TERM unset
	os.Unsetenv("TERM")
	args = buildDockerArgs(cfg, "/tmp/test", "test", proj)
	for i, a := range args {
		if a == "-e" && i+1 < len(args) && strings.HasPrefix(args[i+1], "TERM=") {
			t.Error("TERM should not be in args when TERM is unset")
		}
	}
}

func TestBuildDockerArgs_PortMapping(t *testing.T) {
	for _, port := range []int{3000, 5173, 8080} {
		t.Run(fmt.Sprintf("port_%d", port), func(t *testing.T) {
			cfg := RunConfig{Port: port, Image: "node:22-bookworm-slim"}
			proj := &project.ProjectInfo{RunCmd: "npm run dev"}

			args := buildDockerArgs(cfg, "/tmp/test", "test", proj)
			want := fmt.Sprintf("%d:%d", port, port)
			assertFlagValue(t, args, "-p", want)
		})
	}
}

func TestCheckPort_Free(t *testing.T) {
	// Bind to port 0 to get an OS-assigned free port, then close it,
	// then verify checkPort succeeds on that same port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	if err := checkPort(port); err != nil {
		t.Errorf("checkPort(%d) on free port: %v", port, err)
	}
}

func TestCheckPort_InUse(t *testing.T) {
	// Bind a port and hold it open, then verify checkPort fails.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	err = checkPort(port)
	if err == nil {
		t.Errorf("checkPort(%d) should fail on occupied port", port)
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("error = %q, should mention 'already in use'", err.Error())
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{0, "0ms"},
		{500 * time.Millisecond, "500ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
		{59 * time.Second, "59.0s"},
		{60 * time.Second, "1m0s"},
		{90 * time.Second, "1m30s"},
		{5*time.Minute + 23*time.Second, "5m23s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.dur)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

func TestFormatDockerCmd(t *testing.T) {
	args := []string{
		"run", "--rm", "-it",
		"--name", "pv3-dev-app-abc",
		"--user", "501:20",
		"-v", "/home/user:/workspace:delegated",
		"--cap-drop=ALL",
		"node:22-bookworm-slim",
		"sh", "-c", "npm run dev",
	}

	result := formatDockerCmd("/usr/bin/docker", args)
	lines := strings.Split(result, " \\\n")

	// First line is the binary
	if lines[0] != "/usr/bin/docker" {
		t.Errorf("first line = %q, want '/usr/bin/docker'", lines[0])
	}

	// --name and its value must be on the same line
	foundName := false
	for _, line := range lines {
		if strings.Contains(line, "--name") {
			if !strings.Contains(line, "--name pv3-dev-app-abc") {
				t.Errorf("--name line = %q, want '--name pv3-dev-app-abc' together", line)
			}
			foundName = true
		}
	}
	if !foundName {
		t.Error("--name not found in output")
	}

	// --cap-drop=ALL (flag=value syntax) should be standalone
	foundCap := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "--cap-drop=ALL" {
			foundCap = true
		}
	}
	if !foundCap {
		t.Error("--cap-drop=ALL should be on its own line")
	}

	// sh and -c should be grouped (sh doesn't start with --)
	foundSh := false
	for _, line := range lines {
		if strings.Contains(line, "sh") && strings.Contains(line, "-c") {
			foundSh = true
		}
	}
	if !foundSh {
		t.Error("'sh' and '-c' should be grouped on the same line")
	}
}

// helpers

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	if indexOf(args, want) == -1 {
		t.Errorf("args missing %q", want)
	}
}

func assertNotContains(t *testing.T, args []string, unwanted string) {
	t.Helper()
	if indexOf(args, unwanted) != -1 {
		t.Errorf("args should not contain %q", unwanted)
	}
}

func assertFlagValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			if args[i+1] == value {
				return
			}
			t.Errorf("flag %s = %q, want %q", flag, args[i+1], value)
			return
		}
	}
	t.Errorf("flag %s not found in args", flag)
}

func indexOf(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}
