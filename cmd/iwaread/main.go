package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wmentor/iwaread"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.pages|file.numbers>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	ext := strings.ToLower(filepath.Ext(filename))

	var markdown string
	var err error

	switch ext {
	case ".pages":
		markdown, err = iwaread.ReadPages(filename)
	case ".numbers":
		markdown, err = iwaread.ReadNumbers(filename)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported file type: %s\n", ext)
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(markdown)
}
