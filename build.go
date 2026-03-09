package main

import (
	"bytes"
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

// buildRootToFile streams kustomize build output directly to a file,
// avoiding holding the entire rendered manifest in memory.
func buildRootToFile(dir, outputFile string) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outputFile, err)
	}
	defer f.Close()

	cmd := exec.Command("kustomize", "build", dir)
	cmd.Stdout = f
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Reset file for fallback.
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncating %s: %w", outputFile, err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seeking %s: %w", outputFile, err)
	}
	stderr.Reset()

	cmd = exec.Command("kubectl", "kustomize", dir)
	cmd.Stdout = f
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %s", dir, stderr.String())
	}
	return nil
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

		if outputDir != "" {
			// Stream directly to file to avoid holding manifests in memory.
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("creating output dir: %w", err)
			}
			outFile := filepath.Join(outputDir, sanitizePath(rel))
			if err := buildRootToFile(absPath, outFile); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				failed = true
				continue
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", outFile)
		} else {
			out, err := buildRoot(absPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				failed = true
				continue
			}
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
