package project

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pythonImage = "python:3.12-slim"

// isPythonProject returns true if the directory contains Python project markers.
func isPythonProject(dir string) bool {
	for _, f := range []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"} {
		if fileExists(filepath.Join(dir, f)) {
			return true
		}
	}
	return false
}

// readPythonProject detects the Python framework and dev command.
func readPythonProject(dir string) (*ProjectInfo, error) {
	pm := detectPythonPkgManager(dir)

	// Try pyproject.toml scripts first
	if fileExists(filepath.Join(dir, "pyproject.toml")) {
		sections := parsePyprojectTOML(filepath.Join(dir, "pyproject.toml"))

		// Check [project.scripts] for dev/serve/start/run keys
		if scripts, ok := sections["project.scripts"]; ok {
			if name, cmd := resolvePyScript(scripts); name != "" {
				return &ProjectInfo{
					Runtime:    "python",
					ScriptName: name,
					ScriptCmd:  cmd,
					PkgManager: pm,
					RunCmd:     cmd,
					Image:      pythonImage,
				}, nil
			}
		}

		// Check [tool.poetry.scripts]
		if scripts, ok := sections["tool.poetry.scripts"]; ok {
			if name, cmd := resolvePyScript(scripts); name != "" {
				return &ProjectInfo{
					Runtime:    "python",
					ScriptName: name,
					ScriptCmd:  cmd,
					PkgManager: pm,
					RunCmd:     cmd,
					Image:      pythonImage,
				}, nil
			}
		}
	}

	// Framework detection from dependencies
	deps := collectDependencies(dir)
	if info := detectPythonFramework(dir, deps, pm); info != nil {
		return info, nil
	}

	// Fallback: look for common entry point files
	for _, entry := range []string{"app.py", "main.py"} {
		if fileExists(filepath.Join(dir, entry)) {
			return &ProjectInfo{
				Runtime:    "python",
				ScriptName: entry,
				ScriptCmd:  "python " + entry,
				PkgManager: pm,
				RunCmd:     "python " + entry,
				Image:      pythonImage,
			}, nil
		}
	}

	return nil, fmt.Errorf("no dev command detected for Python project")
}

// resolvePyScript checks a map of script entries for dev/serve/start/run keys.
func resolvePyScript(scripts map[string]string) (string, string) {
	for _, name := range []string{"dev", "serve", "start", "run"} {
		if cmd, ok := scripts[name]; ok && cmd != "" {
			return name, cmd
		}
	}
	return "", ""
}

// parsePyprojectTOML does minimal line-based parsing of pyproject.toml.
// Returns a map of section name -> key-value pairs within that section.
// Handles [section] headers, "key = value" pairs, and inline array dependencies.
func parsePyprojectTOML(path string) map[string]map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	sections := make(map[string]map[string]string)
	var currentSection string
	var inArray bool

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Section header: [project.scripts]
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed[1 : len(trimmed)-1]
			if sections[currentSection] == nil {
				sections[currentSection] = make(map[string]string)
			}
			inArray = false
			continue
		}

		// Skip lines inside multi-line arrays (dependencies lists)
		if inArray {
			if strings.Contains(trimmed, "]") {
				inArray = false
			}
			// Collect dependency names from array items like "django>=5.1",
			if currentSection != "" && strings.HasPrefix(trimmed, "\"") {
				dep := extractDepName(trimmed)
				if dep != "" {
					sections[currentSection]["_dep_"+dep] = dep
				}
			}
			continue
		}

		// Key = value pairs
		if currentSection != "" && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			// Strip surrounding quotes
			val = strings.Trim(val, "\"'")

			// Check if value starts a multi-line array
			if strings.HasPrefix(strings.TrimSpace(parts[1]), "[") && !strings.Contains(parts[1], "]") {
				inArray = true
				continue
			}

			sections[currentSection][key] = val
		}
	}

	return sections
}

// extractDepName pulls the package name from a dependency string like "django>=5.1",
func extractDepName(s string) string {
	s = strings.Trim(s, " \t,\"'")
	// Split on version specifiers
	for _, sep := range []string{">=", "<=", "!=", "==", "~=", ">", "<", "[", " "} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// collectDependencies gathers dependency names from pyproject.toml and requirements.txt.
func collectDependencies(dir string) map[string]bool {
	deps := make(map[string]bool)

	// From pyproject.toml
	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	if fileExists(pyprojectPath) {
		sections := parsePyprojectTOML(pyprojectPath)

		// [project.dependencies]
		for k, v := range sections["project"] {
			if strings.HasPrefix(k, "_dep_") {
				deps[v] = true
			}
		}

		// Also check [project.dependencies] as a section (some formats)
		for k, v := range sections["project.dependencies"] {
			if strings.HasPrefix(k, "_dep_") {
				deps[v] = true
			}
		}

		// [tool.poetry.dependencies]
		for k := range sections["tool.poetry.dependencies"] {
			if k != "python" {
				deps[strings.ToLower(k)] = true
			}
		}
	}

	// From requirements.txt
	reqPath := filepath.Join(dir, "requirements.txt")
	if fileExists(reqPath) {
		for _, dep := range parseRequirementsTxt(reqPath) {
			deps[dep] = true
		}
	}

	return deps
}

// parseRequirementsTxt reads requirement lines and extracts package names.
func parseRequirementsTxt(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var deps []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		name := extractDepName(line)
		if name != "" {
			deps = append(deps, name)
		}
	}
	return deps
}

// detectPythonFramework checks for known frameworks in the dependency set.
func detectPythonFramework(dir string, deps map[string]bool, pm string) *ProjectInfo {
	// Django: needs manage.py
	if deps["django"] && fileExists(filepath.Join(dir, "manage.py")) {
		return &ProjectInfo{
			Runtime:    "python",
			ScriptName: "runserver",
			ScriptCmd:  "python manage.py runserver 0.0.0.0:8000",
			PkgManager: pm,
			RunCmd:     "python manage.py runserver 0.0.0.0:8000",
			Image:      pythonImage,
		}
	}

	// Flask
	if deps["flask"] {
		return &ProjectInfo{
			Runtime:    "python",
			ScriptName: "flask run",
			ScriptCmd:  "flask run --host=0.0.0.0 --port=5000",
			PkgManager: pm,
			RunCmd:     "flask run --host=0.0.0.0 --port=5000",
			Image:      pythonImage,
		}
	}

	// FastAPI
	if deps["fastapi"] {
		return &ProjectInfo{
			Runtime:    "python",
			ScriptName: "uvicorn",
			ScriptCmd:  "uvicorn main:app --host 0.0.0.0 --port 8000 --reload",
			PkgManager: pm,
			RunCmd:     "uvicorn main:app --host 0.0.0.0 --port 8000 --reload",
			Image:      pythonImage,
		}
	}

	return nil
}

// detectPythonPkgManager checks for lockfiles to determine the Python package manager.
func detectPythonPkgManager(dir string) string {
	if fileExists(filepath.Join(dir, "uv.lock")) {
		return "uv"
	}
	if fileExists(filepath.Join(dir, "poetry.lock")) {
		return "poetry"
	}
	if fileExists(filepath.Join(dir, "Pipfile.lock")) || fileExists(filepath.Join(dir, "Pipfile")) {
		return "pipenv"
	}
	return "pip"
}
