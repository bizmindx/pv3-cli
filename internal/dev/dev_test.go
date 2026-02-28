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
		{"image", "node:22-bookworm-slim"},
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
