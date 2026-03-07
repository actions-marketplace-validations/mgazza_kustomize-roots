package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiffResult holds the comparison between two rendered directories.
type DiffResult struct {
	Added    []FileDiff
	Deleted  []FileDiff
	Modified []FileDiff
}

// FileDiff represents a single file's diff information.
type FileDiff struct {
	Name         string
	OldContent   string
	NewContent   string
	LinesAdded   int
	LinesRemoved int
}

// HasChanges reports whether there are any differences.
func (d *DiffResult) HasChanges() bool {
	return len(d.Added) > 0 || len(d.Deleted) > 0 || len(d.Modified) > 0
}

// diffDirs compares files in two directories and returns the differences.
func diffDirs(baseDir, headDir string) (*DiffResult, error) {
	baseFiles, err := readDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading base dir: %w", err)
	}
	headFiles, err := readDir(headDir)
	if err != nil {
		return nil, fmt.Errorf("reading head dir: %w", err)
	}

	result := &DiffResult{}

	// Find modified and deleted files.
	for name, baseContent := range baseFiles {
		headContent, exists := headFiles[name]
		if !exists {
			result.Deleted = append(result.Deleted, FileDiff{
				Name:         name,
				OldContent:   baseContent,
				LinesRemoved: countLines(baseContent),
			})
			continue
		}
		if baseContent != headContent {
			added, removed := countDiffLines(baseContent, headContent)
			result.Modified = append(result.Modified, FileDiff{
				Name:         name,
				OldContent:   baseContent,
				NewContent:   headContent,
				LinesAdded:   added,
				LinesRemoved: removed,
			})
		}
	}

	// Find added files.
	for name, headContent := range headFiles {
		if _, exists := baseFiles[name]; !exists {
			result.Added = append(result.Added, FileDiff{
				Name:       name,
				NewContent: headContent,
				LinesAdded: countLines(headContent),
			})
		}
	}

	// Sort for deterministic output.
	sort.Slice(result.Added, func(i, j int) bool { return result.Added[i].Name < result.Added[j].Name })
	sort.Slice(result.Deleted, func(i, j int) bool { return result.Deleted[i].Name < result.Deleted[j].Name })
	sort.Slice(result.Modified, func(i, j int) bool { return result.Modified[i].Name < result.Modified[j].Name })

	return result, nil
}

// readDir reads all files from a flat directory (matches -output-dir output).
func readDir(dir string) (map[string]string, error) {
	files := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		files[e.Name()] = string(data)
	}
	return files, nil
}

// writeDiffUnified writes a unified diff to the writer.
func writeDiffUnified(w io.Writer, result *DiffResult) {
	for _, f := range result.Deleted {
		fmt.Fprintf(w, "--- a/%s\n+++ /dev/null\n", f.Name)
		for _, line := range strings.Split(strings.TrimRight(f.OldContent, "\n"), "\n") {
			fmt.Fprintf(w, "-%s\n", line)
		}
	}
	for _, f := range result.Added {
		fmt.Fprintf(w, "--- /dev/null\n+++ b/%s\n", f.Name)
		for _, line := range strings.Split(strings.TrimRight(f.NewContent, "\n"), "\n") {
			fmt.Fprintf(w, "+%s\n", line)
		}
	}
	for _, f := range result.Modified {
		fmt.Fprintf(w, "--- a/%s\n+++ b/%s\n", f.Name, f.Name)
		writeUnifiedHunks(w, f.OldContent, f.NewContent)
	}
}

// writeUnifiedHunks writes unified diff hunks with context lines.
func writeUnifiedHunks(w io.Writer, old, new string) {
	oldLines := strings.Split(strings.TrimRight(old, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(new, "\n"), "\n")

	ops := computeEditScript(oldLines, newLines)
	hunks := filterToHunks(ops, 3)

	for _, h := range hunks {
		fmt.Fprintf(w, "@@ -%d,%d +%d,%d @@\n", h.oldStart, h.oldCount, h.newStart, h.newCount)
		for _, op := range h.ops {
			switch op.kind {
			case opEqual:
				fmt.Fprintf(w, " %s\n", op.line)
			case opDelete:
				fmt.Fprintf(w, "-%s\n", op.line)
			case opInsert:
				fmt.Fprintf(w, "+%s\n", op.line)
			}
		}
	}
}

type opKind int

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

type editOp struct {
	kind opKind
	line string
}

// hunk represents a group of changes with surrounding context lines.
type hunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	ops      []editOp
}

// filterToHunks groups changes with surrounding context lines into hunks.
func filterToHunks(ops []editOp, context int) []hunk {
	if len(ops) == 0 {
		return nil
	}

	// Mark which ops are within context range of a change.
	include := make([]bool, len(ops))
	for i, op := range ops {
		if op.kind != opEqual {
			lo := i - context
			if lo < 0 {
				lo = 0
			}
			hi := i + context
			if hi >= len(ops) {
				hi = len(ops) - 1
			}
			for j := lo; j <= hi; j++ {
				include[j] = true
			}
		}
	}

	// Build hunks from consecutive included ops.
	var hunks []hunk
	var cur *hunk
	oldLine, newLine := 1, 1
	for i, op := range ops {
		if include[i] {
			if cur == nil {
				cur = &hunk{oldStart: oldLine, newStart: newLine}
			}
			cur.ops = append(cur.ops, op)
			switch op.kind {
			case opEqual:
				cur.oldCount++
				cur.newCount++
			case opDelete:
				cur.oldCount++
			case opInsert:
				cur.newCount++
			}
		} else if cur != nil {
			hunks = append(hunks, *cur)
			cur = nil
		}

		switch op.kind {
		case opEqual:
			oldLine++
			newLine++
		case opDelete:
			oldLine++
		case opInsert:
			newLine++
		}
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}

	return hunks
}

// computeEditScript produces a minimal edit script between two line slices.
func computeEditScript(a, b []string) []editOp {
	n, m := len(a), len(b)
	// Build LCS table.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce edit script.
	var ops []editOp
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			ops = append(ops, editOp{opEqual, a[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, editOp{opInsert, b[j-1]})
			j--
		} else {
			ops = append(ops, editOp{opDelete, a[i-1]})
			i--
		}
	}

	// Reverse (backtrack produces reversed order).
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

func countLines(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func countDiffLines(old, new string) (added, removed int) {
	ops := computeEditScript(
		strings.Split(strings.TrimRight(old, "\n"), "\n"),
		strings.Split(strings.TrimRight(new, "\n"), "\n"),
	)
	for _, op := range ops {
		switch op.kind {
		case opInsert:
			added++
		case opDelete:
			removed++
		}
	}
	return
}
