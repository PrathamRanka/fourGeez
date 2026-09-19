//go:build ignore

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		fatal(fmt.Errorf("usage: package-lambda <bootstrap> <archive>"))
	}
	input, err := os.Open(os.Args[1])
	if err != nil {
		fatal(err)
	}
	defer input.Close()
	output, err := os.Create(os.Args[2])
	if err != nil {
		fatal(err)
	}
	archive := zip.NewWriter(output)
	header := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := archive.CreateHeader(header)
	if err == nil {
		_, err = io.Copy(entry, input)
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(os.Args[2])
		fatal(err)
	}
	info, err := os.Stat(os.Args[2])
	if err != nil || info.Size() == 0 {
		fatal(fmt.Errorf("Lambda archive was not created"))
	}
	fmt.Printf("created %s (%d bytes) from %s\n", filepath.Clean(os.Args[2]), info.Size(), filepath.Clean(os.Args[1]))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
