package main

import (
	"html/template"
	"io"
	"strings"
)

// k8sMeta holds extracted Kubernetes resource metadata.
type k8sMeta struct {
	Kind      string
	Name      string
	Namespace string
}

// extractK8sMeta extracts kind, name, and namespace from YAML content
// using simple line parsing (avoids a yaml.v3 dependency for this).
func extractK8sMeta(content string) k8sMeta {
	var m k8sMeta
	inMetadata := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "kind:") {
			m.Kind = strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
		}
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}
		if inMetadata {
			if strings.HasPrefix(trimmed, "name:") {
				m.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			}
			if strings.HasPrefix(trimmed, "namespace:") {
				m.Namespace = strings.TrimSpace(strings.TrimPrefix(trimmed, "namespace:"))
			}
			// Exit metadata block when we hit a non-indented line.
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
				inMetadata = false
			}
		}
	}
	return m
}

// htmlFileDiff is the template data for a single file.
type htmlFileDiff struct {
	Name         string
	State        string
	LinesAdded   int
	LinesRemoved int
	Meta         k8sMeta
	DiffLines    []htmlDiffLine
}

// htmlDiffLine is a single line in the diff output.
type htmlDiffLine struct {
	Class   string
	Prefix  string
	Content string
}

// htmlTemplateData is the top-level template data.
type htmlTemplateData struct {
	TotalAdded   int
	TotalRemoved int
	TotalFiles   int
	Files        []htmlFileDiff
}

func buildHTMLData(result *DiffResult) htmlTemplateData {
	var data htmlTemplateData
	data.TotalFiles = len(result.Added) + len(result.Deleted) + len(result.Modified)

	for _, f := range result.Deleted {
		hf := htmlFileDiff{
			Name:         f.Name,
			State:        "Deleted",
			LinesRemoved: f.LinesRemoved,
			Meta:         extractK8sMeta(f.OldContent),
		}
		for _, line := range strings.Split(strings.TrimRight(f.OldContent, "\n"), "\n") {
			hf.DiffLines = append(hf.DiffLines, htmlDiffLine{"removed", "-", line})
		}
		data.TotalRemoved += f.LinesRemoved
		data.Files = append(data.Files, hf)
	}

	for _, f := range result.Added {
		hf := htmlFileDiff{
			Name:       f.Name,
			State:      "Added",
			LinesAdded: f.LinesAdded,
			Meta:       extractK8sMeta(f.NewContent),
		}
		for _, line := range strings.Split(strings.TrimRight(f.NewContent, "\n"), "\n") {
			hf.DiffLines = append(hf.DiffLines, htmlDiffLine{"added", "+", line})
		}
		data.TotalAdded += f.LinesAdded
		data.Files = append(data.Files, hf)
	}

	for _, f := range result.Modified {
		hf := htmlFileDiff{
			Name:         f.Name,
			State:        "Modified",
			LinesAdded:   f.LinesAdded,
			LinesRemoved: f.LinesRemoved,
			Meta:         extractK8sMeta(f.NewContent),
		}
		oldLines := strings.Split(strings.TrimRight(f.OldContent, "\n"), "\n")
		newLines := strings.Split(strings.TrimRight(f.NewContent, "\n"), "\n")
		ops := computeEditScript(oldLines, newLines)
		for _, op := range ops {
			switch op.kind {
			case opEqual:
				hf.DiffLines = append(hf.DiffLines, htmlDiffLine{"unchanged", " ", op.line})
			case opDelete:
				hf.DiffLines = append(hf.DiffLines, htmlDiffLine{"removed", "-", op.line})
			case opInsert:
				hf.DiffLines = append(hf.DiffLines, htmlDiffLine{"added", "+", op.line})
			}
		}
		data.TotalAdded += f.LinesAdded
		data.TotalRemoved += f.LinesRemoved
		data.Files = append(data.Files, hf)
	}

	return data
}

// writeDiffHTML writes an interactive HTML diff page to the writer.
func writeDiffHTML(w io.Writer, result *DiffResult) {
	data := buildHTMLData(result)
	tmpl := template.Must(template.New("diff").Parse(htmlTemplate))
	tmpl.Execute(w, data)
}

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kustomize Roots Diff</title>
<style>
*, *::before, *::after { box-sizing: border-box; }
html, body { margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 14px; color: #24292f; background: #f6f8fa; }
.layout { display: flex; height: 100vh; }
.sidebar { width: 280px; min-width: 200px; background: #fff; border-right: 1px solid #d0d7de; overflow-y: auto; padding: 12px 0; flex-shrink: 0; }
.sidebar-header { padding: 8px 16px; font-weight: 600; font-size: 13px; color: #57606a; border-bottom: 1px solid #d0d7de; margin-bottom: 4px; }
.sidebar a { display: block; padding: 4px 16px; text-decoration: none; color: #24292f; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sidebar a:hover { background: #f3f4f6; }
.sidebar .state-tag { font-size: 11px; padding: 1px 6px; border-radius: 3px; margin-right: 4px; font-weight: 600; }
.sidebar .Added .state-tag { background: #dafbe1; color: #116329; }
.sidebar .Deleted .state-tag { background: #ffebe9; color: #82071e; }
.sidebar .Modified .state-tag { background: #fff8c5; color: #6a5300; }
.main { flex: 1; overflow-y: auto; padding: 16px 24px; }
.summary { background: #fff; border: 1px solid #d0d7de; border-radius: 6px; padding: 12px 16px; margin-bottom: 16px; display: flex; gap: 16px; align-items: center; }
.summary .stat { font-weight: 600; }
.summary .added-stat { color: #116329; }
.summary .removed-stat { color: #82071e; }
.file-card { background: #fff; border: 1px solid #d0d7de; border-radius: 6px; margin-bottom: 16px; overflow: hidden; }
.file-header { background: #f6f8fa; padding: 8px 12px; border-bottom: 1px solid #d0d7de; display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; position: sticky; top: 0; z-index: 1; }
.file-header .filename { font-family: SFMono-Regular, Consolas, monospace; font-size: 13px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.file-header .badge { font-size: 11px; padding: 2px 8px; border-radius: 3px; font-weight: 600; }
.file-header .Added { background: #dafbe1; color: #116329; }
.file-header .Deleted { background: #ffebe9; color: #82071e; }
.file-header .Modified { background: #fff8c5; color: #6a5300; }
.file-header .line-counts { font-size: 12px; white-space: nowrap; }
.file-header .line-counts .plus { color: #116329; }
.file-header .line-counts .minus { color: #82071e; }
.file-header .toggle-arrow { font-size: 12px; color: #57606a; transition: transform 0.15s; }
.file-header .toggle-arrow.collapsed { transform: rotate(-90deg); }
.k8s-meta { background: #f6f8fa; padding: 6px 12px; border-bottom: 1px solid #d0d7de; font-size: 12px; color: #57606a; }
.k8s-meta span { margin-right: 16px; }
.k8s-meta .meta-label { font-weight: 600; color: #24292f; }
.diff-content { font-family: SFMono-Regular, Consolas, monospace; font-size: 12px; line-height: 20px; overflow-x: auto; }
.diff-content .diff-line { display: flex; white-space: pre; }
.diff-content .line-prefix { width: 20px; text-align: center; flex-shrink: 0; user-select: none; }
.diff-content .line-text { flex: 1; padding: 0 8px; }
.diff-content .added { background: #dafbe1; }
.diff-content .added .line-prefix { background: #ccffd8; color: #116329; }
.diff-content .removed { background: #ffebe9; }
.diff-content .removed .line-prefix { background: #ffd7d5; color: #82071e; }
.diff-content .unchanged { background: #fff; }
.diff-content .unchanged .line-prefix { color: #8b949e; }
.collapsed-content { display: none; }
.toolbar { padding: 8px 16px; border-bottom: 1px solid #d0d7de; display: flex; gap: 8px; }
.toolbar button { font-size: 12px; padding: 4px 12px; border: 1px solid #d0d7de; border-radius: 4px; background: #fff; cursor: pointer; color: #24292f; }
.toolbar button:hover { background: #f3f4f6; }
@media (max-width: 768px) { .sidebar { display: none; } }
</style>
</head>
<body>
<div class="layout">
  <div class="sidebar">
    <div class="sidebar-header">Files changed ({{.TotalFiles}})</div>
    {{range $i, $f := .Files}}
    <a href="#file-{{$i}}" class="{{$f.State}}"><span class="state-tag">{{$f.State}}</span>{{$f.Name}}</a>
    {{end}}
  </div>
  <div class="main">
    <div class="summary">
      <span class="stat">{{.TotalFiles}} file{{if ne .TotalFiles 1}}s{{end}} changed</span>
      <span class="stat added-stat">+{{.TotalAdded}}</span>
      <span class="stat removed-stat">-{{.TotalRemoved}}</span>
    </div>
    <div class="toolbar">
      <button onclick="toggleAll(true)">Expand all</button>
      <button onclick="toggleAll(false)">Collapse all</button>
      <button onclick="toggleSidebar()">Toggle sidebar</button>
    </div>
    {{range $i, $f := .Files}}
    <div class="file-card" id="file-{{$i}}">
      <div class="file-header" onclick="toggleFile(this)">
        <span class="toggle-arrow">&#9660;</span>
        <span class="filename">{{$f.Name}}</span>
        <span class="badge {{$f.State}}">{{$f.State}}</span>
        <span class="line-counts">
          {{if gt $f.LinesAdded 0}}<span class="plus">+{{$f.LinesAdded}}</span>{{end}}
          {{if gt $f.LinesRemoved 0}}<span class="minus">-{{$f.LinesRemoved}}</span>{{end}}
        </span>
      </div>
      {{if or (ne $f.Meta.Kind "") (ne $f.Meta.Name "")}}
      <div class="k8s-meta">
        {{if ne $f.Meta.Kind ""}}<span><span class="meta-label">Kind:</span> {{$f.Meta.Kind}}</span>{{end}}
        {{if ne $f.Meta.Name ""}}<span><span class="meta-label">Name:</span> {{$f.Meta.Name}}</span>{{end}}
        {{if ne $f.Meta.Namespace ""}}<span><span class="meta-label">Namespace:</span> {{$f.Meta.Namespace}}</span>{{end}}
      </div>
      {{end}}
      <div class="diff-content">
        {{range $f.DiffLines}}
        <div class="diff-line {{.Class}}"><span class="line-prefix">{{.Prefix}}</span><span class="line-text">{{.Content}}</span></div>
        {{end}}
      </div>
    </div>
    {{end}}
  </div>
</div>
<script>
function toggleFile(header) {
  var content = header.parentElement.querySelector('.diff-content');
  var meta = header.parentElement.querySelector('.k8s-meta');
  var arrow = header.querySelector('.toggle-arrow');
  if (content.classList.contains('collapsed-content')) {
    content.classList.remove('collapsed-content');
    if (meta) meta.classList.remove('collapsed-content');
    arrow.classList.remove('collapsed');
  } else {
    content.classList.add('collapsed-content');
    if (meta) meta.classList.add('collapsed-content');
    arrow.classList.add('collapsed');
  }
}
function toggleAll(expand) {
  var contents = document.querySelectorAll('.diff-content');
  var metas = document.querySelectorAll('.k8s-meta');
  var arrows = document.querySelectorAll('.toggle-arrow');
  for (var i = 0; i < contents.length; i++) {
    if (expand) { contents[i].classList.remove('collapsed-content'); }
    else { contents[i].classList.add('collapsed-content'); }
  }
  for (var i = 0; i < metas.length; i++) {
    if (expand) { metas[i].classList.remove('collapsed-content'); }
    else { metas[i].classList.add('collapsed-content'); }
  }
  for (var i = 0; i < arrows.length; i++) {
    if (expand) { arrows[i].classList.remove('collapsed'); }
    else { arrows[i].classList.add('collapsed'); }
  }
}
function toggleSidebar() {
  var sb = document.querySelector('.sidebar');
  sb.style.display = sb.style.display === 'none' ? '' : 'none';
}
document.querySelectorAll('.sidebar a').forEach(function(a) {
  a.addEventListener('click', function(e) {
    var target = document.querySelector(this.getAttribute('href'));
    if (target) {
      var content = target.querySelector('.diff-content');
      if (content && content.classList.contains('collapsed-content')) {
        toggleFile(target.querySelector('.file-header'));
      }
    }
  });
});
</script>
</body>
</html>`
