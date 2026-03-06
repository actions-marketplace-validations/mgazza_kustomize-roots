# kustomize-roots

Find the root kustomization files in a directory tree — the ones that nothing else references.

Kustomize projects have a tree of `kustomization.yaml` files referencing each other via `resources`, `components`, and `bases`. When validating in CI, you only want to build the "roots" — kustomizations that nothing else references. Building every `kustomization.yaml` is wasteful and many intermediates don't build standalone.

## Install

```bash
go install github.com/mgazza/kustomize-roots@latest
```

## Usage

```
kustomize-roots [flags] [directory]
```

### Flags

| Flag | Description |
|------|-------------|
| `-build` | Build each root via `kustomize build` (falls back to `kubectl kustomize`) |
| `-output-dir` | Write build output to files instead of stdout |
| `-json` | Output root paths as JSON array |
| `-verbose` | Print reference graph to stderr |
| `-exclude` | Glob patterns to skip directories (repeatable) |

### Examples

List all root kustomizations:

```bash
kustomize-roots /path/to/repo
```

Output as JSON:

```bash
kustomize-roots -json /path/to/repo
```

Build all roots and write to a directory:

```bash
kustomize-roots -build -output-dir ./rendered /path/to/repo
```

Exclude directories:

```bash
kustomize-roots -exclude .git -exclude vendor /path/to/repo
```

### Diff subcommand

Compare two directories of rendered manifests:

```bash
# Unified diff
kustomize-roots diff /tmp/base-rendered/ /tmp/head-rendered/

# HTML diff (interactive standalone viewer)
kustomize-roots diff -format html /tmp/base/ /tmp/head/ > diff.html
```

The diff compares files by name across both directories, reporting added, deleted, and modified files with LCS-based line diffing.

## GitHub Action

Use in your workflows to find or validate kustomize roots:

```yaml
- uses: mgazza/kustomize-roots@main
  id: roots
  with:
    directory: .
    exclude: ".git vendor"

- run: echo "${{ steps.roots.outputs.roots }}"
```

Build all roots to validate they render cleanly:

```yaml
- uses: mgazza/kustomize-roots@main
  with:
    directory: .
    build: "true"
    output-dir: ./rendered
    exclude: ".git"
```

### Artifact mode

Generate an interactive HTML diff and upload as a GitHub Actions artifact. Posts a summary comment on the PR:

```yaml
- uses: mgazza/kustomize-roots@main
  with:
    mode: artifact
    exclude: ".git src"
```

Requires `contents: read` and `pull-requests: write` permissions.

### PR-branch mode

Maintain an orphan `rendered` branch with rendered manifests tracking main. Creates shadow PRs so reviewers see the rendered diff in GitHub's native Files Changed tab:

```yaml
on:
  pull_request:
    types: [opened, synchronize, closed]

concurrency:
  group: kustomize-diff-${{ github.event.pull_request.number }}
  cancel-in-progress: true

jobs:
  rendered-diff:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: mgazza/kustomize-roots@main
        with:
          mode: pr-branch
          exclude: ".git src"
```

When a source PR is opened/updated, a shadow PR is created on the `rendered` branch showing the rendered manifest diff. When the source PR is merged, the shadow PR is auto-merged to keep the `rendered` branch in sync. When closed without merge, the shadow PR is cleaned up.

### Action inputs

| Input | Default | Description |
|-------|---------|-------------|
| `directory` | `.` | Directory to scan |
| `build` | `false` | Build each root via kustomize build |
| `output-dir` | | Write build output to files |
| `json` | `false` | Output as JSON array |
| `exclude` | `.git` | Space-separated glob patterns to skip |
| `mode` | `build` | Operating mode: `build`, `artifact`, or `pr-branch` |
| `base-ref` | | Base git ref for diff modes (defaults to PR base) |
| `rendered-branch` | `rendered` | Orphan branch name for pr-branch mode |

### Action outputs

| Output | Description |
|--------|-------------|
| `roots` | Newline-separated list of root paths |
| `roots-json` | JSON array of root paths |
| `diff` | Unified diff text (artifact/pr-branch modes) |
| `has-changes` | Whether rendered manifests changed |
| `shadow-pr` | Shadow PR number (pr-branch mode only) |

## How it works

1. **Discover** — walks the directory tree finding all `kustomization.yaml`, `kustomization.yml`, and `Kustomization` files
2. **Parse** — reads `resources`, `components`, and `bases` fields from each file
3. **Graph** — builds a reference graph, resolving relative paths and skipping remote refs
4. **Roots** — identifies nodes with in-degree zero (not referenced by any other kustomization)

## License

MIT
