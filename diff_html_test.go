package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteDiffHTML_ContainsDoctype(t *testing.T) {
	result := &DiffResult{
		Modified: []FileDiff{{
			Name:       "clusters_dev.yaml",
			OldContent: "image: app:v1\n",
			NewContent: "image: app:v2\n",
			LinesAdded: 1, LinesRemoved: 1,
		}},
	}

	var buf bytes.Buffer
	writeDiffHTML(&buf, result)
	html := buf.String()

	if !strings.Contains(html, "<!doctype html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(html, "clusters_dev.yaml") {
		t.Error("expected filename in output")
	}
}

func TestWriteDiffHTML_ShowsAddedDeleted(t *testing.T) {
	result := &DiffResult{
		Added: []FileDiff{{
			Name:       "clusters_staging.yaml",
			NewContent: "apiVersion: v1\n",
			LinesAdded: 1,
		}},
		Deleted: []FileDiff{{
			Name:         "clusters_old.yaml",
			OldContent:   "apiVersion: v1\n",
			LinesRemoved: 1,
		}},
	}

	var buf bytes.Buffer
	writeDiffHTML(&buf, result)
	html := buf.String()

	if !strings.Contains(html, "clusters_staging.yaml") {
		t.Error("expected added file in output")
	}
	if !strings.Contains(html, "clusters_old.yaml") {
		t.Error("expected deleted file in output")
	}
	if !strings.Contains(html, "Added") {
		t.Error("expected Added state label")
	}
	if !strings.Contains(html, "Deleted") {
		t.Error("expected Deleted state label")
	}
}

func TestWriteDiffHTML_K8sMetadata(t *testing.T) {
	result := &DiffResult{
		Modified: []FileDiff{{
			Name:       "clusters_dev.yaml",
			OldContent: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx\n  namespace: default\n",
			NewContent: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx\n  namespace: default\nspec:\n  replicas: 3\n",
			LinesAdded: 2, LinesRemoved: 0,
		}},
	}

	var buf bytes.Buffer
	writeDiffHTML(&buf, result)
	html := buf.String()

	if !strings.Contains(html, "Deployment") {
		t.Error("expected Kind in metadata")
	}
	if !strings.Contains(html, "nginx") {
		t.Error("expected Name in metadata")
	}
}

func TestWriteDiffHTML_Standalone(t *testing.T) {
	// Verify no external dependencies (all CSS/JS inline).
	result := &DiffResult{
		Modified: []FileDiff{{
			Name: "test.yaml", OldContent: "a\n", NewContent: "b\n",
			LinesAdded: 1, LinesRemoved: 1,
		}},
	}

	var buf bytes.Buffer
	writeDiffHTML(&buf, result)
	html := buf.String()

	if strings.Contains(html, "cdnjs.cloudflare.com") {
		t.Error("HTML should be standalone with no external CDN links")
	}
	if !strings.Contains(html, "<style>") {
		t.Error("expected inline CSS")
	}
}
