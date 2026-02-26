package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// --- Unit tests: graph logic with in-memory nodes/edges ---

func TestFindRoots_SimpleChain(t *testing.T) {
	// A -> B -> C, root = A
	nodes := map[string]struct{}{
		"A": {},
		"B": {},
		"C": {},
	}
	edges := map[string][]string{
		"A": {"B"},
		"B": {"C"},
	}
	roots := findRootsFromNodes(nodes, edges)
	expected := []string{"A"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestFindRoots_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D, root = A
	nodes := map[string]struct{}{
		"A": {},
		"B": {},
		"C": {},
		"D": {},
	}
	edges := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
	}
	roots := findRootsFromNodes(nodes, edges)
	expected := []string{"A"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestFindRoots_Disconnected(t *testing.T) {
	// A -> B, C -> D, roots = [A, C]
	nodes := map[string]struct{}{
		"A": {},
		"B": {},
		"C": {},
		"D": {},
	}
	edges := map[string][]string{
		"A": {"B"},
		"C": {"D"},
	}
	roots := findRootsFromNodes(nodes, edges)
	expected := []string{"A", "C"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestFindRoots_SingleNode(t *testing.T) {
	nodes := map[string]struct{}{
		"A": {},
	}
	edges := map[string][]string{}
	roots := findRootsFromNodes(nodes, edges)
	expected := []string{"A"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestIsRemoteRef(t *testing.T) {
	tests := []struct {
		ref    string
		remote bool
	}{
		{"../base", false},
		{"./overlay", false},
		{"components/foo", false},
		{"https://github.com/org/repo//path", true},
		{"github.com/org/repo//path?ref=v1.0", true},
		{"github.com/org/repo", true},
		{"gitlab.com/org/repo", true},
		{"ssh://git@github.com/org/repo", true},
		{"git::https://example.com/repo.git", true},
	}
	for _, tt := range tests {
		got := isRemoteRef(tt.ref)
		if got != tt.remote {
			t.Errorf("isRemoteRef(%q) = %v, want %v", tt.ref, got, tt.remote)
		}
	}
}

// --- Integration tests: real filesystem ---

func writeKustomization(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_SimpleHierarchy(t *testing.T) {
	tmp := t.TempDir()

	// Create a structure mimicking:
	// clusters/local-dev -> components/agent, components/cp
	// components/agent -> base/agent
	// components/cp -> base/cp
	// base/agent (leaf)
	// base/cp (leaf)

	writeKustomization(t, filepath.Join(tmp, "clusters", "local-dev"), `
resources:
  - ../../components/agent
  - ../../components/cp
`)
	writeKustomization(t, filepath.Join(tmp, "components", "agent"), `
resources:
  - ../../base/agent
`)
	writeKustomization(t, filepath.Join(tmp, "components", "cp"), `
resources:
  - ../../base/cp
`)
	writeKustomization(t, filepath.Join(tmp, "base", "agent"), `
resources:
  - deployment.yaml
`)
	writeKustomization(t, filepath.Join(tmp, "base", "cp"), `
resources:
  - deployment.yaml
`)

	nodes, err := discover(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(nodes))
	}

	g := buildGraph(nodes)
	roots := findRoots(g, tmp)

	expected := []string{"clusters/local-dev"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestIntegration_MultipleRoots(t *testing.T) {
	tmp := t.TempDir()

	// Two independent root clusters, sharing a base.
	writeKustomization(t, filepath.Join(tmp, "clusters", "dev"), `
resources:
  - ../../base
`)
	writeKustomization(t, filepath.Join(tmp, "clusters", "prod"), `
resources:
  - ../../base
`)
	writeKustomization(t, filepath.Join(tmp, "base"), `
resources:
  - deployment.yaml
`)

	nodes, err := discover(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}

	g := buildGraph(nodes)
	roots := findRoots(g, tmp)

	expected := []string{"clusters/dev", "clusters/prod"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestIntegration_WithComponents(t *testing.T) {
	tmp := t.TempDir()

	writeKustomization(t, filepath.Join(tmp, "app"), `
resources:
  - ../base
components:
  - ../components/monitoring
`)
	writeKustomization(t, filepath.Join(tmp, "base"), `
resources:
  - deployment.yaml
`)
	writeKustomization(t, filepath.Join(tmp, "components", "monitoring"), `
resources:
  - servicemonitor.yaml
`)

	nodes, err := discover(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}

	g := buildGraph(nodes)
	roots := findRoots(g, tmp)

	expected := []string{"app"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestIntegration_ExcludePattern(t *testing.T) {
	tmp := t.TempDir()

	writeKustomization(t, filepath.Join(tmp, "clusters", "dev"), `
resources:
  - ../../base
`)
	writeKustomization(t, filepath.Join(tmp, "clusters", "staging"), `
resources:
  - ../../base
`)
	writeKustomization(t, filepath.Join(tmp, "base"), `
resources:
  - deployment.yaml
`)

	nodes, err := discover(tmp, []string{"staging"})
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(nodes), nodes)
	}

	g := buildGraph(nodes)
	roots := findRoots(g, tmp)

	expected := []string{"clusters/dev"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestIntegration_RemoteRefsIgnored(t *testing.T) {
	tmp := t.TempDir()

	writeKustomization(t, filepath.Join(tmp, "app"), `
resources:
  - https://github.com/org/repo//manifests?ref=v1.0
  - ../base
`)
	writeKustomization(t, filepath.Join(tmp, "base"), `
resources:
  - deployment.yaml
`)

	nodes, err := discover(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}

	g := buildGraph(nodes)
	roots := findRoots(g, tmp)

	expected := []string{"app"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}

func TestIntegration_KustomizationYml(t *testing.T) {
	tmp := t.TempDir()

	// Use kustomization.yml instead of kustomization.yaml
	dir := filepath.Join(tmp, "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yml"), []byte(`
resources:
  - ../base
`), 0o644); err != nil {
		t.Fatal(err)
	}

	writeKustomization(t, filepath.Join(tmp, "base"), `
resources:
  - deployment.yaml
`)

	nodes, err := discover(tmp, nil)
	if err != nil {
		t.Fatal(err)
	}

	g := buildGraph(nodes)
	roots := findRoots(g, tmp)

	expected := []string{"app"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("got %v, want %v", roots, expected)
	}
}
