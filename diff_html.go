package main

import "io"

// writeDiffHTML writes an interactive HTML diff page to the writer.
func writeDiffHTML(w io.Writer, result *DiffResult) {
	// TODO: implement in Task 2
	writeDiffUnified(w, result)
}
