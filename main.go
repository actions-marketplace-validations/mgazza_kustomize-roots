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
	var (
		buildFlag   = flag.Bool("build", false, "Build each root via kustomize build")
		outputDir   = flag.String("output-dir", "", "Write build output to files instead of stdout")
		jsonFlag    = flag.Bool("json", false, "Output root paths as JSON array")
		verboseFlag = flag.Bool("verbose", false, "Print reference graph to stderr")
		excludes    excludeFlags
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
