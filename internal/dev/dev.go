package dev

import (
	"fmt"
	"os"

	"github.com/pv3dev/pv3/internal/docker"
	"github.com/spf13/cobra"
)

func NewDevCmd() *cobra.Command {
	var cfg docker.RunConfig

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run your dev server in an isolated Docker container",
		Long: `Reads package.json, detects your package manager, and runs your dev script
inside a secure Docker container with live terminal output and hot reload.

Feels exactly like running natively — same logs, same localhost URLs.
The only difference: rogue dependencies can't touch your host system.`,
		Example: `  pv3 dev                        # auto-detect and run dev script
  pv3 dev --port 3000            # use port 3000 instead of 5173
  pv3 dev --no-net               # fully offline, no network access
  pv3 dev --image node:20-slim   # override the default image
  pv3 dev --verbose              # show the docker run command`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return docker.Run(cfg)
		},
	}

	cmd.Flags().IntVar(&cfg.Port, "port", 5173, "Host port to publish")
	cmd.Flags().BoolVar(&cfg.NoNet, "no-net", false, "Disable all network access")
	cmd.Flags().StringVar(&cfg.Image, "image", "", "Container image (auto-detected from project type)")
	cmd.Flags().BoolVar(&cfg.Verbose, "verbose", false, "Print the docker run command")

	return cmd
}

func NewInstallCmd() *cobra.Command {
	var cfg docker.RunConfig

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install dependencies inside an isolated Docker container",
		Long: `Detects your project type, spins up a sandboxed Docker container,
and runs the appropriate install command (npm install, pip install, cargo fetch, etc.).

Postinstall scripts and build hooks execute inside the container —
they cannot access your host system beyond the project directory.`,
		Example: `  pv3 install                     # auto-detect and install deps
  pv3 install --no-net            # install with no network (from cache)
  pv3 install --image node:20-slim  # override the default image
  pv3 install --verbose           # show the docker run command`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return docker.RunInstall(cfg)
		},
	}

	cmd.Flags().BoolVar(&cfg.NoNet, "no-net", false, "Disable all network access")
	cmd.Flags().StringVar(&cfg.Image, "image", "", "Container image (auto-detected from project type)")
	cmd.Flags().BoolVar(&cfg.Verbose, "verbose", false, "Print the docker run command")

	return cmd
}

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "pv3",
		Short:   "Run code safely — local Docker sandbox or cloud microVM",
		Version: version,
		Long: `pv3 is the secure default way to run code.

Use "pv3 dev" for daily development with hot reload in a local Docker sandbox.
Use "pv3 run" for maximum isolation in a cloud microVM.`,
	}

	root.AddCommand(NewDevCmd())
	root.AddCommand(NewInstallCmd())

	return root
}

func Execute(version string) {
	if err := NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
