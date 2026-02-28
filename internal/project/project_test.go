package project

import (
	"os"
	"path/filepath"
	"testing"
)

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

			got := detectPackageManager(dir)
			if got != tt.wantPM {
				t.Errorf("detectPackageManager() = %q, want %q", got, tt.wantPM)
			}
		})
	}
}

func TestReadProject(t *testing.T) {
	tests := []struct {
		name        string
		packageJSON string
		lockfiles   []string
		wantScript  string
		wantPM      string
		wantRunCmd  string
		wantErr     bool
	}{
		{
			name:        "vite project with npm",
			packageJSON: `{"scripts":{"dev":"vite","build":"tsc && vite build"}}`,
			wantScript:  "dev",
			wantPM:      "npm",
			wantRunCmd:  "npm run dev",
		},
		{
			name:        "next.js project with yarn",
			packageJSON: `{"scripts":{"dev":"next dev","build":"next build","start":"next start"}}`,
			lockfiles:   []string{"yarn.lock"},
			wantScript:  "dev",
			wantPM:      "yarn",
			wantRunCmd:  "yarn run dev",
		},
		{
			name:        "nuxt project with pnpm",
			packageJSON: `{"scripts":{"dev":"nuxt dev"}}`,
			lockfiles:   []string{"pnpm-lock.yaml"},
			wantScript:  "dev",
			wantPM:      "pnpm",
			wantRunCmd:  "pnpm run dev",
		},
		{
			name:        "only start script",
			packageJSON: `{"scripts":{"start":"node server.js"}}`,
			wantScript:  "start",
			wantPM:      "npm",
			wantRunCmd:  "npm run start",
		},
		{
			name:        "no scripts at all",
			packageJSON: `{"name":"empty-project"}`,
			wantErr:     true,
		},
		{
			name:        "invalid json",
			packageJSON: `{broken json`,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(tt.packageJSON), 0644); err != nil {
				t.Fatal(err)
			}
			for _, f := range tt.lockfiles {
				if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0644); err != nil {
					t.Fatal(err)
				}
			}

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
			if info.ScriptName != tt.wantScript {
				t.Errorf("ScriptName = %q, want %q", info.ScriptName, tt.wantScript)
			}
			if info.PkgManager != tt.wantPM {
				t.Errorf("PkgManager = %q, want %q", info.PkgManager, tt.wantPM)
			}
			if info.RunCmd != tt.wantRunCmd {
				t.Errorf("RunCmd = %q, want %q", info.RunCmd, tt.wantRunCmd)
			}
		})
	}
}

func TestReadProject_NoPackageJSON(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadProject(dir)
	if err == nil {
		t.Fatal("expected error for missing package.json")
	}
	want := "no package.json found in current directory"
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}
