package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildRoot runs kustomize build on a single root directory.
// It tries `kustomize build` first, falling back to `kubectl kustomize`.
func buildRoot(dir string) ([]byte, error) {
	out, err := exec.Command("kustomize", "build", dir).CombinedOutput()
	if err == nil {
		return out, nil
	}

	// Fallback to kubectl kustomize.
	out, err = exec.Command("kubectl", "kustomize", dir).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("building %s: %s", dir, string(out))
	}
	return out, nil
}

// sanitizePath converts a relative path to a safe filename.
func sanitizePath(rel string) string {
	s := strings.ReplaceAll(rel, string(filepath.Separator), "_")
	s = strings.ReplaceAll(s, ".", "_")
	if s == "" {
		s = "root"
	}
	return s + ".yaml"
}

// buildRoots builds each root kustomization and writes output.
func buildRoots(roots []string, baseDir, outputDir string) error {
	var failed bool
	for i, rel := range roots {
		absPath := filepath.Join(baseDir, rel)
		out, err := buildRoot(absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			failed = true
			continue
		}

		if outputDir != "" {
			outFile := filepath.Join(outputDir, sanitizePath(rel))
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("creating output dir: %w", err)
			}
			if err := os.WriteFile(outFile, out, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", outFile, err)
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", outFile)
		} else {
			if i > 0 {
				fmt.Println("---")
			}
			fmt.Print(string(out))
		}
	}
	if failed {
		return fmt.Errorf("one or more builds failed")
	}
	return nil
}
