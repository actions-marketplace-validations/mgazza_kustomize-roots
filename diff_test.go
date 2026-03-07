package main

import (
	"bytes"
	"fmt"
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

func TestWriteUnifiedHunks_ExcludesDistantLines(t *testing.T) {
	// 20-line file with only line 10 changed — lines far from the change must be excluded.
	var oldLines, newLines []string
	for i := 1; i <= 20; i++ {
		oldLines = append(oldLines, fmt.Sprintf("line %d", i))
		if i == 10 {
			newLines = append(newLines, "line 10 modified")
		} else {
			newLines = append(newLines, fmt.Sprintf("line %d", i))
		}
	}

	old := strings.Join(oldLines, "\n") + "\n"
	new := strings.Join(newLines, "\n") + "\n"

	var buf bytes.Buffer
	writeUnifiedHunks(&buf, old, new)
	output := buf.String()

	// Should contain hunk header.
	if !strings.Contains(output, "@@ -7,7 +7,7 @@") {
		t.Errorf("expected hunk header '@@ -7,7 +7,7 @@', got:\n%s", output)
	}

	// Should NOT contain distant unchanged lines.
	if strings.Contains(output, " line 1\n") {
		t.Error("line 1 should be excluded (too far from change)")
	}
	if strings.Contains(output, " line 5\n") {
		t.Error("line 5 should be excluded (too far from change)")
	}
	if strings.Contains(output, " line 15\n") {
		t.Error("line 15 should be excluded (too far from change)")
	}
	if strings.Contains(output, " line 20\n") {
		t.Error("line 20 should be excluded (too far from change)")
	}

	// Should contain context and changed lines.
	if !strings.Contains(output, " line 7\n") {
		t.Error("context line 7 should be included")
	}
	if !strings.Contains(output, " line 9\n") {
		t.Error("context line 9 should be included")
	}
	if !strings.Contains(output, "-line 10\n") {
		t.Error("removed line 10 should be included")
	}
	if !strings.Contains(output, "+line 10 modified\n") {
		t.Error("added line 10 modified should be included")
	}
	if !strings.Contains(output, " line 13\n") {
		t.Error("context line 13 should be included")
	}
}

func TestFilterToHunks_MergesOverlappingContext(t *testing.T) {
	// Two changes 4 lines apart — with 3 context lines they should merge into one hunk.
	var oldLines, newLines []string
	for i := 1; i <= 15; i++ {
		oldLines = append(oldLines, fmt.Sprintf("line %d", i))
		if i == 5 || i == 10 {
			newLines = append(newLines, fmt.Sprintf("line %d changed", i))
		} else {
			newLines = append(newLines, fmt.Sprintf("line %d", i))
		}
	}

	ops := computeEditScript(oldLines, newLines)
	hunks := filterToHunks(ops, 3)

	// Changes at lines 5 and 10 with 3 context: ranges overlap so should be 1 hunk.
	if len(hunks) != 1 {
		t.Fatalf("expected 1 merged hunk, got %d", len(hunks))
	}
}

func TestFilterToHunks_SplitsSeparateChanges(t *testing.T) {
	// Two changes far apart — should produce 2 separate hunks.
	var oldLines, newLines []string
	for i := 1; i <= 30; i++ {
		oldLines = append(oldLines, fmt.Sprintf("line %d", i))
		if i == 5 || i == 25 {
			newLines = append(newLines, fmt.Sprintf("line %d changed", i))
		} else {
			newLines = append(newLines, fmt.Sprintf("line %d", i))
		}
	}

	ops := computeEditScript(oldLines, newLines)
	hunks := filterToHunks(ops, 3)

	if len(hunks) != 2 {
		t.Fatalf("expected 2 separate hunks, got %d", len(hunks))
	}
}
