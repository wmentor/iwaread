package iwaread

import (
	"fmt"
	"os"
	"testing"
)

func TestReadPages(t *testing.T) {
	// Find a .pages fixture file
	fixturePath := "testdata/go-magnetar.pages"
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("fixture file not found at %s", fixturePath)
	}

	md, err := ReadPages(fixturePath)
	if err != nil {
		t.Fatalf("ReadPages failed: %v", err)
	}

	if md == "" {
		t.Error("expected non-empty markdown output")
	}

	fmt.Println(md)

	t.Logf("Successfully extracted %d characters of markdown", len(md))

	// Check that we have some expected content
	if len(md) < 1000 {
		t.Errorf("expected at least 1000 characters, got %d", len(md))
	}
}
