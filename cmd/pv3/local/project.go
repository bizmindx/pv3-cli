package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProjectInfo struct {
	ScriptName string // "dev", "start", or "serve"
	ScriptCmd  string // the actual script value from package.json
	PkgManager string // "npm", "yarn", or "pnpm"
	RunCmd     string // full command: e.g. "npm run dev"
}

type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

// ReadProject reads package.json and lockfiles from dir to determine
// what dev command to run and which package manager to use.
func ReadProject(dir string) (*ProjectInfo, error) {
	pkgPath := filepath.Join(dir, "package.json")

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no package.json found in current directory")
		}
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

	pm := detectPackageManager(dir)

	return &ProjectInfo{
		ScriptName: scriptName,
		ScriptCmd:  scriptCmd,
		PkgManager: pm,
		RunCmd:     fmt.Sprintf("%s run %s", pm, scriptName),
	}, nil
}

// resolveScript picks the first available script in priority order.
func resolveScript(scripts map[string]string) (string, string, error) {
	for _, name := range []string{"dev", "start", "serve"} {
		if cmd, ok := scripts[name]; ok && cmd != "" {
			return name, cmd, nil
		}
	}
	return "", "", fmt.Errorf("no dev, start, or serve script found in package.json")
}

// detectPackageManager checks for lockfiles to determine the package manager.
func detectPackageManager(dir string) string {
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
