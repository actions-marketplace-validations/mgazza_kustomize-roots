package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiffFiles_NoChanges(t *testing.T) {
	base := t.TempDir()
	head := t.TempDir()

	writeFile(t, base, "clusters_dev.yaml", "apiVersion: v1\nkind: Namespace\n")
	writeFile(t, head, "clusters_dev.yaml", "apiVersion: v1\nkind: Namespace\n")

	var buf bytes.Buffer
	result, err := diffDirs(base, head)
	if err != nil {
		t.Fatal(err)
	}
	writeDiffUnified(&buf, result)
	if buf.Len() != 0 {
		t.Errorf("expected empty diff, got:\n%s", buf.String())
	}
}

func TestDiffFiles_Modified(t *testing.T) {
	base := t.TempDir()
	head := t.TempDir()

	writeFile(t, base, "clusters_dev.yaml", "image: app:v1\n")
	writeFile(t, head, "clusters_dev.yaml", "image: app:v2\n")

	result, err := diffDirs(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Modified) != 1 {
		t.Fatalf("expected 1 modified, got %d", len(result.Modified))
	}
	if result.Modified[0].Name != "clusters_dev.yaml" {
		t.Errorf("expected clusters_dev.yaml, got %s", result.Modified[0].Name)
	}

	var buf bytes.Buffer
	writeDiffUnified(&buf, result)
	output := buf.String()
	if !strings.Contains(output, "-image: app:v1") {
		t.Errorf("expected removed line in diff:\n%s", output)
	}
	if !strings.Contains(output, "+image: app:v2") {
		t.Errorf("expected added line in diff:\n%s", output)
	}
}

func TestDiffFiles_Added(t *testing.T) {
	base := t.TempDir()
	head := t.TempDir()

	writeFile(t, head, "clusters_staging.yaml", "apiVersion: v1\n")

	result, err := diffDirs(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(result.Added))
	}
}

func TestDiffFiles_Deleted(t *testing.T) {
	base := t.TempDir()
	head := t.TempDir()

	writeFile(t, base, "clusters_old.yaml", "apiVersion: v1\n")

	result, err := diffDirs(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(result.Deleted))
	}
}
