package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRustProject_SolanaAnchor(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "rust", "solana-anchor")
	info, err := readRustProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "rust")
	assertField(t, "ScriptName", info.ScriptName, "anchor build")
	assertField(t, "PkgManager", info.PkgManager, "cargo")
	assertField(t, "RunCmd", info.RunCmd, "cargo run")
	assertField(t, "Image", info.Image, rustImage)
}

func TestReadRustProject_SolanaNative(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "rust", "solana-native")
	info, err := readRustProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "rust")
	assertField(t, "ScriptName", info.ScriptName, "cargo build-sbf")
	assertField(t, "PkgManager", info.PkgManager, "cargo")
	assertField(t, "RunCmd", info.RunCmd, "cargo run")
	assertField(t, "Image", info.Image, rustImage)
}

func TestReadRustProject_Generic(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "rust", "rust-generic")
	info, err := readRustProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "rust")
	assertField(t, "ScriptName", info.ScriptName, "run")
	assertField(t, "RunCmd", info.RunCmd, "cargo run")
	assertField(t, "PkgManager", info.PkgManager, "cargo")
	assertField(t, "Image", info.Image, rustImage)
}

func TestReadRustProject_NoDev(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "rust", "rust-no-dev")
	_, err := readRustProject(dir)
	if err == nil {
		t.Error("expected error for Rust project with no package section")
	}
}

func TestIsRustProject(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"Cargo.toml", []string{"Cargo.toml"}, true},
		{"empty directory", []string{}, false},
		{"only package.json", []string{"package.json"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte("[package]\nname = \"test\"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := isRustProject(dir)
			if got != tt.want {
				t.Errorf("isRustProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectRustDependencies(t *testing.T) {
	dir := t.TempDir()
	cargo := `[package]
name = "test"
version = "0.1.0"

[dependencies]
anchor-lang = "0.30.1"
solana-sdk = "2.1"
serde = { version = "1.0", features = ["derive"] }
`
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644); err != nil {
		t.Fatal(err)
	}

	deps := collectRustDependencies(dir)

	for _, want := range []string{"anchor-lang", "solana-sdk", "serde"} {
		if !deps[want] {
			t.Errorf("missing dependency %q", want)
		}
	}
}

func TestDetectRustFramework(t *testing.T) {
	tests := []struct {
		name       string
		deps       map[string]bool
		anchorToml bool
		wantScript string
		wantNil    bool
	}{
		{
			name:       "anchor-lang dependency",
			deps:       map[string]bool{"anchor-lang": true},
			wantScript: "anchor build",
		},
		{
			name:       "Anchor.toml present",
			deps:       map[string]bool{},
			anchorToml: true,
			wantScript: "anchor build",
		},
		{
			name:       "solana-sdk dependency",
			deps:       map[string]bool{"solana-sdk": true},
			wantScript: "cargo build-sbf",
		},
		{
			name:       "solana-program dependency",
			deps:       map[string]bool{"solana-program": true},
			wantScript: "cargo build-sbf",
		},
		{
			name:    "no framework",
			deps:    map[string]bool{"serde": true},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.anchorToml {
				if err := os.WriteFile(filepath.Join(dir, "Anchor.toml"), []byte("[provider]\ncluster = \"localnet\"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			info := detectRustFramework(dir, tt.deps)
			if tt.wantNil {
				if info != nil {
					t.Errorf("expected nil, got %+v", info)
				}
				return
			}
			if info == nil {
				t.Fatal("expected non-nil ProjectInfo")
			}
			assertField(t, "ScriptName", info.ScriptName, tt.wantScript)
			assertField(t, "Runtime", info.Runtime, "rust")
		})
	}
}
