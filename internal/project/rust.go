package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const rustImage = "rust:1.85-slim"

// isRustProject returns true if the directory contains a Cargo.toml.
func isRustProject(dir string) bool {
	return fileExists(filepath.Join(dir, "Cargo.toml"))
}

// readRustProject detects the Rust framework and dev command.
func readRustProject(dir string) (*ProjectInfo, error) {
	cargoPath := filepath.Join(dir, "Cargo.toml")

	deps := collectRustDependencies(dir)

	// Framework detection from dependencies
	if info := detectRustFramework(dir, deps); info != nil {
		return info, nil
	}

	// Check for [package] section to confirm it's a valid project
	sections := parseCargoTOML(cargoPath)
	if sections == nil {
		return nil, fmt.Errorf("reading Cargo.toml: file is empty or unreadable")
	}

	// Workspace without detectable framework
	if _, hasWorkspace := sections["workspace"]; hasWorkspace {
		if _, hasPkg := sections["package"]; !hasPkg {
			return nil, fmt.Errorf("no dev command detected for Rust project")
		}
	}

	if _, hasPkg := sections["package"]; !hasPkg {
		return nil, fmt.Errorf("no dev command detected for Rust project")
	}

	return &ProjectInfo{
		Runtime:    "rust",
		ScriptName: "run",
		ScriptCmd:  "cargo run",
		PkgManager: "cargo",
		RunCmd:     "cargo run",
		Image:      rustImage,
	}, nil
}

// parseCargoTOML does minimal line-based parsing of Cargo.toml.
// Returns a map of section name -> key-value pairs.
func parseCargoTOML(path string) map[string]map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	sections := make(map[string]map[string]string)
	var currentSection string

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Section header: [dependencies]
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed[1 : len(trimmed)-1]
			if sections[currentSection] == nil {
				sections[currentSection] = make(map[string]string)
			}
			continue
		}

		// Key = value pairs
		if currentSection != "" && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			sections[currentSection][key] = val
		}
	}

	return sections
}

// collectRustDependencies gathers dependency names from Cargo.toml.
func collectRustDependencies(dir string) map[string]bool {
	deps := make(map[string]bool)

	cargoPath := filepath.Join(dir, "Cargo.toml")
	if !fileExists(cargoPath) {
		return deps
	}

	sections := parseCargoTOML(cargoPath)
	for key := range sections["dependencies"] {
		deps[strings.ToLower(key)] = true
	}

	// Also check workspace dependencies
	for key := range sections["workspace.dependencies"] {
		deps[strings.ToLower(key)] = true
	}

	return deps
}

// detectRustFramework checks for known frameworks in the dependency set.
func detectRustFramework(dir string, deps map[string]bool) *ProjectInfo {
	// Anchor (Solana): anchor-lang dependency + Anchor.toml
	if deps["anchor-lang"] || fileExists(filepath.Join(dir, "Anchor.toml")) {
		return &ProjectInfo{
			Runtime:    "rust",
			ScriptName: "anchor build",
			ScriptCmd:  "anchor build",
			PkgManager: "cargo",
			RunCmd:     "cargo run",
			Image:      rustImage,
		}
	}

	// Native Solana: solana-sdk or solana-program
	if deps["solana-sdk"] || deps["solana-program"] {
		return &ProjectInfo{
			Runtime:    "rust",
			ScriptName: "cargo build-sbf",
			ScriptCmd:  "cargo build-sbf",
			PkgManager: "cargo",
			RunCmd:     "cargo run",
			Image:      rustImage,
		}
	}

	return nil
}
