# kustomize-roots Diff Modes Design

## Goal

Add two diff modes to kustomize-roots so that PRs changing kustomize manifests show the rendered impact — either as a rich HTML artifact or as a native GitHub diff via shadow PRs.

## Background

kustomize-roots currently discovers root kustomization files and optionally builds them. The missing piece: when a PR changes kustomize source, reviewers can't see what the rendered manifests will actually look like after the change. They review patches and overlays but not the final output.

Vitrifi's borg repo solved this with `lawgiver generatediffs` + an HTML template (`diff-page.tmpl`) that generates an interactive diff viewer uploaded as a CI artifact. We want to bring this capability to kustomize-roots as a reusable GitHub Action, with two modes.

## Architecture

### Core diff pipeline (shared by both modes)

1. Checkout base ref (e.g. `origin/main`)
2. Run `kustomize-roots -build -output-dir /tmp/base-rendered/` — discovers roots and renders them
3. Checkout head ref (PR branch)
4. Run `kustomize-roots -build -output-dir /tmp/head-rendered/` — renders PR state
5. Diff the two output directories

### Mode 1: Artifact (`mode: artifact`)

- Generate a rich, interactive HTML diff page from the two directories
- Upload as a GitHub Actions artifact
- Post a PR comment with:
  - Summary: roots affected, files changed count, additions/deletions
  - Link to download/view the artifact
- Self-contained, no git operations beyond checkout

### Mode 2: PR Branch (`mode: pr-branch`)

- Maintain an orphan branch `rendered` holding rendered manifests from main
- On source PR opened/updated:
  1. Create/update branch `rendered/pr-<number>` from `rendered`
  2. Write head-rendered output into it, commit, push
  3. Create shadow PR: `rendered` <- `rendered/pr-<number>` (or update if exists)
  4. Comment on source PR linking to the shadow PR
- On source PR merged:
  1. Merge the shadow PR (updates `rendered` to match new main state)
  2. Delete `rendered/pr-<number>` branch
- On source PR closed without merge:
  1. Close shadow PR
  2. Delete `rendered/pr-<number>` branch

The shadow PRs are the mechanism for keeping the `rendered` branch in sync with `main`. Merging a source PR triggers merging its shadow, so `rendered` always reflects the latest main.

### Lifecycle diagram

```
Source PR opened/updated:
  main ──build──> rendered (orphan branch, base)
  PR   ──build──> rendered/pr-123 (shadow branch, head)
  Shadow PR created: rendered <- rendered/pr-123
  Comment on source PR: "View rendered diff: #shadow-pr-url"

Source PR merged:
  Shadow PR auto-merged -> rendered branch updated
  rendered/pr-123 branch deleted

Source PR closed:
  Shadow PR closed
  rendered/pr-123 branch deleted
```

## Changes to kustomize-roots

### CLI additions

New flag or subcommand for diffing two output directories:

```bash
# Diff two rendered directories
kustomize-roots diff /tmp/base-rendered/ /tmp/head-rendered/

# Output as unified diff
kustomize-roots diff -format unified /tmp/base/ /tmp/head/

# Output as HTML (using built-in template)
kustomize-roots diff -format html /tmp/base/ /tmp/head/ > diff.html
```

The diff logic:
- Match files by name across both directories
- Files in head but not base = added
- Files in base but not head = deleted
- Files in both = compare content, report if modified
- For HTML output: parse YAML to extract K8s metadata (kind, name, namespace) for richer display

### GitHub Action additions

New inputs:

| Input | Default | Description |
|-------|---------|-------------|
| `mode` | `build` | `build` (existing), `artifact`, `pr-branch` |
| `base-ref` | auto | Base ref for diff (defaults to PR base branch) |
| `rendered-branch` | `rendered` | Orphan branch name for pr-branch mode |

Existing inputs unchanged: `directory`, `build`, `output-dir`, `exclude`, `json`.

New outputs:

| Output | Description |
|--------|-------------|
| `diff` | Unified diff text (artifact/pr-branch modes) |
| `shadow-pr` | Shadow PR number (pr-branch mode only) |
| `has-changes` | Boolean — whether rendered output changed |

### HTML template

Inspired by vitrifi's `diff-page.tmpl`:
- Interactive file tree sidebar
- Side-by-side diff view with syntax highlighting
- Color-coded additions/deletions
- K8s resource metadata (kind, name, namespace, GVK)
- Collapsible sections per file
- Standalone HTML (no external dependencies, inline CSS/JS)

## Implementation order

1. CLI `diff` subcommand (unified output)
2. CLI `diff -format html` (HTML template)
3. Action `mode: artifact` (render + diff + upload + comment)
4. Action `mode: pr-branch` (orphan branch + shadow PR lifecycle)
5. Adopt in testkube-workspace
6. Adopt in bng-edge-infra

## Consumers

- **testkube-workspace** (kubeshop) — 23 kustomization files, GitHub
- **bng-edge-infra** (codelaboratoryltd) — 35 kustomization files, GitHub
