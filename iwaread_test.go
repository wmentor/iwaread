package iwaread

import (
	"fmt"
	"os"
	"strings"
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

func TestReadNumbers(t *testing.T) {
	// Find a .numbers fixture file
	fixturePath := "testdata/earnings.numbers"
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("fixture file not found at %s", fixturePath)
	}

	md, err := ReadNumbers(fixturePath)
	if err != nil {
		t.Fatalf("ReadNumbers failed: %v", err)
	}

	if md == "" {
		t.Error("expected non-empty markdown output")
	}

	fmt.Println(md)

	t.Logf("Successfully extracted %d characters of markdown", len(md))

	// Check that we have H1 headings (table names)
	if !strings.Contains(md, "# ") {
		t.Error("expected markdown output to contain H1 headings")
	}

	// Check that we have table separators
	if !strings.Contains(md, "| --- |") {
		t.Error("expected markdown output to contain table separators")
	}
}

func TestReadNumbersBlank(t *testing.T) {
	// Test with a blank document
	fixturePath := "testdata/blank.numbers"
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("fixture file not found at %s", fixturePath)
	}

	md, err := ReadNumbers(fixturePath)
	if err != nil {
		t.Fatalf("ReadNumbers failed: %v", err)
	}

	if md == "" {
		t.Error("expected non-empty markdown output")
	}

	// Should have at least one table heading
	if !strings.Contains(md, "# Table 1") {
		t.Error("expected markdown output to contain '# Table 1' heading")
	}
}
