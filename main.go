package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type excludeFlags []string

func (e *excludeFlags) String() string { return strings.Join(*e, ",") }
func (e *excludeFlags) Set(v string) error {
	*e = append(*e, v)
	return nil
}

func main() {
	// Check for diff subcommand.
	if len(os.Args) > 1 && os.Args[1] == "diff" {
		runDiff(os.Args[2:])
		return
	}

	var (
		buildFlag    = flag.Bool("build", false, "Build each root via kustomize build")
		outputDir    = flag.String("output-dir", "", "Write build output to files instead of stdout")
		changedFiles = flag.String("changed-files", "", "Space-separated changed file paths — only build roots affected by these files")
		jsonFlag     = flag.Bool("json", false, "Output root paths as JSON array")
		verboseFlag  = flag.Bool("verbose", false, "Print reference graph to stderr")
		excludes     excludeFlags
	)
	flag.Var(&excludes, "exclude", "Glob patterns to skip directories (repeatable)")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	nodes, err := discover(dir, excludes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	g := buildGraph(nodes)

	if *verboseFlag {
		for src, targets := range g.edges {
			for _, t := range targets {
				fmt.Fprintf(os.Stderr, "%s -> %s\n", src, t)
			}
		}
	}

	roots := findRoots(g, dir)

	if *changedFiles != "" {
		roots = g.affectedRoots(strings.Fields(*changedFiles), dir)
		if len(roots) == 0 {
			fmt.Fprintln(os.Stderr, "no roots affected by changed files")
			return
		}
	}

	if *buildFlag {
		if err := buildRoots(roots, dir, *outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(roots); err != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
			os.Exit(1)
		}
		return
	}

	for _, r := range roots {
		fmt.Println(r)
	}
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	format := fs.String("format", "unified", "Output format: unified, html")
	fs.Parse(args)

	if fs.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "usage: kustomize-roots diff [-format unified|html] <base-dir> <head-dir>\n")
		os.Exit(1)
	}

	baseDir := fs.Arg(0)
	headDir := fs.Arg(1)

	result, err := diffDirs(baseDir, headDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	if !result.HasChanges() {
		fmt.Fprintln(os.Stderr, "no changes")
		return
	}

	switch *format {
	case "unified":
		writeDiffUnified(os.Stdout, result)
	case "html":
		writeDiffHTML(os.Stdout, result)
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s\n", *format)
		os.Exit(1)
	}
}
