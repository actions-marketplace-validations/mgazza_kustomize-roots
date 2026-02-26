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

### Action inputs

| Input | Default | Description |
|-------|---------|-------------|
| `directory` | `.` | Directory to scan |
| `build` | `false` | Build each root via kustomize build |
| `output-dir` | | Write build output to files |
| `json` | `false` | Output as JSON array |
| `exclude` | `.git` | Space-separated glob patterns to skip |

### Action outputs

| Output | Description |
|--------|-------------|
| `roots` | Newline-separated list of root paths |
| `roots-json` | JSON array of root paths |

## How it works

1. **Discover** — walks the directory tree finding all `kustomization.yaml`, `kustomization.yml`, and `Kustomization` files
2. **Parse** — reads `resources`, `components`, and `bases` fields from each file
3. **Graph** — builds a reference graph, resolving relative paths and skipping remote refs
4. **Roots** — identifies nodes with in-degree zero (not referenced by any other kustomization)

## License

MIT
