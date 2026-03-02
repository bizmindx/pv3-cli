package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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

func TestResolveScript(t *testing.T) {
	tests := []struct {
		name     string
		scripts  map[string]string
		wantName string
		wantCmd  string
		wantErr  bool
	}{
		{
			name:     "dev script found",
			scripts:  map[string]string{"dev": "vite", "build": "tsc"},
			wantName: "dev",
			wantCmd:  "vite",
		},
		{
			name:     "start script fallback",
			scripts:  map[string]string{"start": "node server.js", "build": "tsc"},
			wantName: "start",
			wantCmd:  "node server.js",
		},
		{
			name:     "serve script fallback",
			scripts:  map[string]string{"serve": "vue-cli-service serve"},
			wantName: "serve",
			wantCmd:  "vue-cli-service serve",
		},
		{
			name:     "dev takes priority over start",
			scripts:  map[string]string{"dev": "next dev", "start": "next start"},
			wantName: "dev",
			wantCmd:  "next dev",
		},
		{
			name:    "no matching script",
			scripts: map[string]string{"build": "tsc", "test": "jest"},
			wantErr: true,
		},
		{
			name:    "nil scripts",
			scripts: nil,
			wantErr: true,
		},
		{
			name:     "empty dev script ignored",
			scripts:  map[string]string{"dev": "", "start": "node index.js"},
			wantName: "start",
			wantCmd:  "node index.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, cmd, err := resolveScript(tt.scripts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("script name = %q, want %q", name, tt.wantName)
			}
			if cmd != tt.wantCmd {
				t.Errorf("script cmd = %q, want %q", cmd, tt.wantCmd)
			}
		})
	}
}

func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		wantPM string
	}{
		{
			name:   "pnpm from lockfile",
			files:  []string{"pnpm-lock.yaml"},
			wantPM: "pnpm",
		},
		{
			name:   "yarn from lockfile",
			files:  []string{"yarn.lock"},
			wantPM: "yarn",
		},
		{
			name:   "npm by default",
			files:  []string{},
			wantPM: "npm",
		},
		{
			name:   "pnpm takes priority over yarn",
			files:  []string{"pnpm-lock.yaml", "yarn.lock"},
			wantPM: "pnpm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := detectNodePkgManager(dir)
			if got != tt.wantPM {
				t.Errorf("detectNodePkgManager() = %q, want %q", got, tt.wantPM)
			}
		})
	}
}

// TestReadProject uses testdata fixtures that represent realistic mini-projects.
func TestReadProject(t *testing.T) {
	td := testdataDir(t)

	tests := []struct {
		name        string
		fixture     string // path relative to testdata/
		wantRuntime string
		wantScript  string
		wantPM      string
		wantRunCmd  string
		wantImage   string
		wantErr     bool
	}{
		{
			name:        "vite-react with npm",
			fixture:     "javascript/vite-react",
			wantRuntime: "node",
			wantScript:  "dev",
			wantPM:      "npm",
			wantRunCmd:  "npm run dev",
			wantImage:   "node:22-bookworm-slim",
		},
		{
			name:        "next.js with yarn",
			fixture:     "javascript/nextjs-yarn",
			wantRuntime: "node",
			wantScript:  "dev",
			wantPM:      "yarn",
			wantRunCmd:  "yarn run dev",
			wantImage:   "node:22-bookworm-slim",
		},
		{
			name:        "nuxt with pnpm",
			fixture:     "javascript/nuxt-pnpm",
			wantRuntime: "node",
			wantScript:  "dev",
			wantPM:      "pnpm",
			wantRunCmd:  "pnpm run dev",
			wantImage:   "node:22-bookworm-slim",
		},
		{
			name:        "express with start only",
			fixture:     "javascript/express-start",
			wantRuntime: "node",
			wantScript:  "start",
			wantPM:      "npm",
			wantRunCmd:  "npm run start",
			wantImage:   "node:22-bookworm-slim",
		},
		{
			name:        "vue-cli with serve only",
			fixture:     "javascript/vue-serve",
			wantRuntime: "node",
			wantScript:  "serve",
			wantPM:      "npm",
			wantRunCmd:  "npm run serve",
			wantImage:   "node:22-bookworm-slim",
		},
		{
			name:    "no dev script",
			fixture: "javascript/no-dev-script",
			wantErr: true,
		},
		{
			name:    "invalid json",
			fixture: "javascript/invalid-json",
			wantErr: true,
		},
		{
			name:        "project with .env file",
			fixture:     "javascript/env-file",
			wantRuntime: "node",
			wantScript:  "dev",
			wantPM:      "npm",
			wantRunCmd:  "npm run dev",
			wantImage:   "node:22-bookworm-slim",
		},
		{
			name:        "django via ReadProject dispatcher",
			fixture:     "python/python-django",
			wantRuntime: "python",
			wantScript:  "runserver",
			wantPM:      "pip",
			wantRunCmd:  "python manage.py runserver 0.0.0.0:8000",
			wantImage:   "python:3.12-slim",
		},
		{
			name:        "flask via ReadProject dispatcher",
			fixture:     "python/python-flask",
			wantRuntime: "python",
			wantScript:  "flask run",
			wantPM:      "pip",
			wantRunCmd:  "flask run --host=0.0.0.0 --port=5000",
			wantImage:   "python:3.12-slim",
		},
		{
			name:        "solana anchor via ReadProject dispatcher",
			fixture:     "rust/solana-anchor",
			wantRuntime: "rust",
			wantScript:  "anchor build",
			wantPM:      "cargo",
			wantRunCmd:  "cargo run",
			wantImage:   "rust:1.85-slim",
		},
		{
			name:        "generic rust via ReadProject dispatcher",
			fixture:     "rust/rust-generic",
			wantRuntime: "rust",
			wantScript:  "run",
			wantPM:      "cargo",
			wantRunCmd:  "cargo run",
			wantImage:   "rust:1.85-slim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(td, tt.fixture)

			info, err := ReadProject(dir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Runtime != tt.wantRuntime {
				t.Errorf("Runtime = %q, want %q", info.Runtime, tt.wantRuntime)
			}
			if info.ScriptName != tt.wantScript {
				t.Errorf("ScriptName = %q, want %q", info.ScriptName, tt.wantScript)
			}
			if info.PkgManager != tt.wantPM {
				t.Errorf("PkgManager = %q, want %q", info.PkgManager, tt.wantPM)
			}
			if info.RunCmd != tt.wantRunCmd {
				t.Errorf("RunCmd = %q, want %q", info.RunCmd, tt.wantRunCmd)
			}
			if info.Image != tt.wantImage {
				t.Errorf("Image = %q, want %q", info.Image, tt.wantImage)
			}
		})
	}
}

func TestReadProject_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadProject(dir)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
	want := "could not detect project type in current directory"
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

// TestReadProject_UnsupportedRuntime verifies that projects without
// Node.js or Python markers are rejected cleanly.
func TestReadProject_UnsupportedRuntime(t *testing.T) {
	td := testdataDir(t)

	dir := filepath.Join(td, "go", "go-project")
	_, err := ReadProject(dir)
	if err == nil {
		t.Error("expected error for Go project (unsupported runtime), got nil")
	}
}
