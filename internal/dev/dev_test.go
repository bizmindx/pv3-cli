package dev

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewDevCmd_Defaults(t *testing.T) {
	cmd := NewDevCmd()

	checks := []struct {
		flag string
		want interface{}
	}{
		{"port", 5173},
		{"no-net", false},
		{"image", ""},
		{"verbose", false},
	}

	for _, c := range checks {
		t.Run(c.flag, func(t *testing.T) {
			switch want := c.want.(type) {
			case int:
				got, err := cmd.Flags().GetInt(c.flag)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("--%s = %d, want %d", c.flag, got, want)
				}
			case bool:
				got, err := cmd.Flags().GetBool(c.flag)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("--%s = %v, want %v", c.flag, got, want)
				}
			case string:
				got, err := cmd.Flags().GetString(c.flag)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("--%s = %q, want %q", c.flag, got, want)
				}
			}
		})
	}
}

func TestNewDevCmd_FlagParsing(t *testing.T) {
	cmd := NewDevCmd()
	cmd.SetArgs([]string{"--port", "3000", "--no-net", "--image", "node:20-slim", "--verbose"})
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	port, _ := cmd.Flags().GetInt("port")
	if port != 3000 {
		t.Errorf("port = %d, want 3000", port)
	}

	noNet, _ := cmd.Flags().GetBool("no-net")
	if !noNet {
		t.Error("no-net should be true after --no-net")
	}

	image, _ := cmd.Flags().GetString("image")
	if image != "node:20-slim" {
		t.Errorf("image = %q, want 'node:20-slim'", image)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if !verbose {
		t.Error("verbose should be true after --verbose")
	}
}

func TestNewDevCmd_InvalidPort(t *testing.T) {
	cmd := NewDevCmd()
	cmd.SetArgs([]string{"--port", "not-a-number"})
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-numeric port")
	}
}

func TestNewRootCmd_Version(t *testing.T) {
	root := NewRootCmd("1.2.3")
	if root.Version != "1.2.3" {
		t.Errorf("version = %q, want '1.2.3'", root.Version)
	}
}

func TestNewRootCmd_DevSubcommand(t *testing.T) {
	root := NewRootCmd("test")

	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Name())
	}

	found := false
	for _, n := range names {
		if n == "dev" {
			found = true
		}
	}
	if !found {
		t.Errorf("subcommands = %v, missing 'dev'", names)
	}
}

func TestNewInstallCmd_Defaults(t *testing.T) {
	cmd := NewInstallCmd()

	noNet, err := cmd.Flags().GetBool("no-net")
	if err != nil {
		t.Fatal(err)
	}
	if noNet {
		t.Error("no-net should default to false")
	}

	image, err := cmd.Flags().GetString("image")
	if err != nil {
		t.Fatal(err)
	}
	if image != "" {
		t.Errorf("image should default to empty, got %q", image)
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		t.Fatal(err)
	}
	if verbose {
		t.Error("verbose should default to false")
	}
}

func TestNewInstallCmd_FlagParsing(t *testing.T) {
	cmd := NewInstallCmd()
	cmd.SetArgs([]string{"--no-net", "--image", "node:20-slim", "--verbose"})
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	noNet, _ := cmd.Flags().GetBool("no-net")
	if !noNet {
		t.Error("no-net should be true after --no-net")
	}

	image, _ := cmd.Flags().GetString("image")
	if image != "node:20-slim" {
		t.Errorf("image = %q, want 'node:20-slim'", image)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if !verbose {
		t.Error("verbose should be true after --verbose")
	}
}

func TestNewInstallCmd_NoPortFlag(t *testing.T) {
	cmd := NewInstallCmd()
	_, err := cmd.Flags().GetInt("port")
	if err == nil {
		t.Error("install command should not have a --port flag")
	}
}

func TestNewRootCmd_InstallSubcommand(t *testing.T) {
	root := NewRootCmd("test")

	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Name())
	}

	found := false
	for _, n := range names {
		if n == "install" {
			found = true
		}
	}
	if !found {
		t.Errorf("subcommands = %v, missing 'install'", names)
	}
}

func TestNewRunCmd_Defaults(t *testing.T) {
	cmd := NewRunCmd()

	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		t.Fatal(err)
	}
	if port != 5173 {
		t.Errorf("port = %d, want 5173", port)
	}

	noNet, _ := cmd.Flags().GetBool("no-net")
	if noNet {
		t.Error("no-net should default to false")
	}

	image, _ := cmd.Flags().GetString("image")
	if image != "" {
		t.Errorf("image should default to empty, got %q", image)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		t.Error("verbose should default to false")
	}
}

func TestNewRunCmd_RequiresArg(t *testing.T) {
	cmd := NewRunCmd()
	cmd.SetArgs([]string{})
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no repo URL provided")
	}
}

func TestNewRunCmd_FlagParsing(t *testing.T) {
	cmd := NewRunCmd()
	cmd.SetArgs([]string{"https://github.com/user/repo", "--port", "3000", "--verbose"})
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	port, _ := cmd.Flags().GetInt("port")
	if port != 3000 {
		t.Errorf("port = %d, want 3000", port)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if !verbose {
		t.Error("verbose should be true after --verbose")
	}
}

func TestNewRootCmd_RunSubcommand(t *testing.T) {
	root := NewRootCmd("test")

	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Name())
	}

	found := false
	for _, n := range names {
		if n == "run" {
			found = true
		}
	}
	if !found {
		t.Errorf("subcommands = %v, missing 'run'", names)
	}
}
