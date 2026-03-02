package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProjectInfo struct {
	Runtime    string // "node", "python", or "rust"
	ScriptName string // "dev", "start", "runserver", etc.
	ScriptCmd  string // the actual script value or resolved command
	PkgManager string // "npm", "yarn", "pnpm", "pip", "uv", "poetry"
	RunCmd     string // full command to execute in container
	Image      string // default Docker image for this runtime
}

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

// ReadProject detects the project type and returns the dev command to run.
// Detection order: Node.js (package.json) → Python → Rust → error.
func ReadProject(dir string) (*ProjectInfo, error) {
	if fileExists(filepath.Join(dir, "package.json")) {
		return readNodeProject(dir)
	}

	if isPythonProject(dir) {
		return readPythonProject(dir)
	}

	if isRustProject(dir) {
		return readRustProject(dir)
	}

	return nil, fmt.Errorf("could not detect project type in current directory")
}

// readNodeProject reads package.json and lockfiles from dir.
func readNodeProject(dir string) (*ProjectInfo, error) {
	pkgPath := filepath.Join(dir, "package.json")

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("reading package.json: %w", err)
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package.json: %w", err)
	}

	scriptName, scriptCmd, err := resolveScript(pkg.Scripts)
	if err != nil {
		return nil, err
	}

	pm := detectNodePkgManager(dir)

	return &ProjectInfo{
		Runtime:    "node",
		ScriptName: scriptName,
		ScriptCmd:  scriptCmd,
		PkgManager: pm,
		RunCmd:     fmt.Sprintf("%s run %s", pm, scriptName),
		Image:      "node:22-bookworm-slim",
	}, nil
}

func resolveScript(scripts map[string]string) (string, string, error) {
	for _, name := range []string{"dev", "start", "serve"} {
		if cmd, ok := scripts[name]; ok && cmd != "" {
			return name, cmd, nil
		}
	}
	return "", "", fmt.Errorf("no dev, start, or serve script found in package.json")
}

func detectNodePkgManager(dir string) string {
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return "yarn"
	}
	return "npm"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
