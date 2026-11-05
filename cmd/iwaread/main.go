package main

import (
	"fmt"
	"log"
	"os"

	"github.com/wmentor/iwaread"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.pages>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	markdown, err := iwaread.ReadPages(filename)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(markdown)
}
