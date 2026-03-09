package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// kustomizationFileNames are the recognized kustomization file names.
var kustomizationFileNames = []string{
	"kustomization.yaml",
	"kustomization.yml",
	"Kustomization",
}

// kustomization is a minimal representation of a kustomization file.
type kustomization struct {
	Kind       string   `yaml:"kind"`
	Resources  []string `yaml:"resources"`
	Components []string `yaml:"components"`
	Bases      []string `yaml:"bases"`
}

// graph represents the kustomization reference graph.
type graph struct {
	// nodes maps absolute directory path to the kustomization file path.
	nodes map[string]string
	// edges maps a kustomization dir to the dirs it references.
	edges map[string][]string
	// inDegree tracks how many times each node is referenced.
	inDegree map[string]int
}

// discover walks the directory tree and finds all kustomization files.
func discover(root string, excludes []string) (map[string]string, error) {
	nodes := make(map[string]string)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(absRoot, path)
			for _, pattern := range excludes {
				matched, matchErr := filepath.Match(pattern, filepath.Base(path))
				if matchErr == nil && matched {
					return filepath.SkipDir
				}
				// Also try matching against relative path.
				matched, matchErr = filepath.Match(pattern, rel)
				if matchErr == nil && matched {
					return filepath.SkipDir
				}
			}
			return nil
		}
		dir := filepath.Dir(path)
		if _, exists := nodes[dir]; exists {
			return nil
		}
		for _, name := range kustomizationFileNames {
			if d.Name() == name {
				nodes[dir] = path
				return nil
			}
		}
		return nil
	})
	return nodes, err
}

// isRemoteRef returns true if the reference looks like a remote URL.
func isRemoteRef(ref string) bool {
	if strings.Contains(ref, "://") {
		return true
	}
	if strings.Contains(ref, "?ref=") || strings.Contains(ref, "?version=") {
		return true
	}
	// GitHub/GitLab shorthand: github.com/..., gitlab.com/...
	if strings.HasPrefix(ref, "github.com/") || strings.HasPrefix(ref, "gitlab.com/") {
		return true
	}
	return false
}

// references returns all local directory references from a kustomization.
func references(k kustomization) []string {
	var refs []string
	for _, r := range k.Resources {
		if !isRemoteRef(r) {
			refs = append(refs, r)
		}
	}
	for _, c := range k.Components {
		if !isRemoteRef(c) {
			refs = append(refs, c)
		}
	}
	for _, b := range k.Bases {
		if !isRemoteRef(b) {
			refs = append(refs, b)
		}
	}
	return refs
}

// buildGraph constructs a reference graph from discovered kustomizations.
func buildGraph(nodes map[string]string) *graph {
	g := &graph{
		nodes:    nodes,
		edges:    make(map[string][]string),
		inDegree: make(map[string]int),
	}

	// Initialize in-degree for all nodes.
	for dir := range nodes {
		g.inDegree[dir] = 0
	}

	for dir, filePath := range nodes {
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: reading %s: %v\n", filePath, err)
			continue
		}

		var k kustomization
		if err := yaml.Unmarshal(data, &k); err != nil {
			fmt.Fprintf(os.Stderr, "warning: parsing %s: %v\n", filePath, err)
			continue
		}

		// Components are not standalone roots — remove them from the graph.
		if k.Kind == "Component" {
			delete(g.nodes, dir)
			delete(g.inDegree, dir)
			continue
		}

		for _, ref := range references(k) {
			target := filepath.Join(dir, ref)
			target = filepath.Clean(target)
			if _, exists := nodes[target]; exists {
				g.edges[dir] = append(g.edges[dir], target)
				g.inDegree[target]++
			}
		}
	}

	return g
}

// findRoots returns the root kustomization directories (in-degree == 0), sorted.
func findRoots(g *graph, root string) []string {
	absRoot, _ := filepath.Abs(root)
	var roots []string
	for dir, deg := range g.inDegree {
		if deg == 0 {
			rel, err := filepath.Rel(absRoot, dir)
			if err != nil {
				rel = dir
			}
			roots = append(roots, rel)
		}
	}
	sort.Strings(roots)
	return roots
}

// findRootsFromNodes is a helper for testing that works with pre-built graph data.
func findRootsFromNodes(nodes map[string]struct{}, edges map[string][]string) []string {
	inDegree := make(map[string]int)
	for n := range nodes {
		inDegree[n] = 0
	}
	for _, targets := range edges {
		for _, t := range targets {
			if _, ok := nodes[t]; ok {
				inDegree[t]++
			}
		}
	}

	var roots []string
	for n, deg := range inDegree {
		if deg == 0 {
			roots = append(roots, n)
		}
	}
	sort.Strings(roots)
	return roots
}
