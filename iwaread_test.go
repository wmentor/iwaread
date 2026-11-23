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

func TestReadKeynote(t *testing.T) {
	// Find a .key fixture file
	fixturePath := "testdata/test.key"
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("fixture file not found at %s", fixturePath)
	}

	md, err := ReadKeynote(fixturePath)
	if err != nil {
		t.Fatalf("ReadKeynote failed: %v", err)
	}

	if md == "" {
		t.Error("expected non-empty markdown output")
	}

	fmt.Println(md)

	t.Logf("Successfully extracted %d characters of markdown", len(md))

	// Check that we have slide headings
	if !strings.Contains(md, "## Slide") {
		t.Error("expected markdown output to contain slide headings")
	}

	// Check that we have the expected title text from first slide
	if !strings.Contains(md, "A sample presentation") {
		t.Error("expected markdown output to contain 'A sample presentation'")
	}

	// Check that we have the expected body text from first slide
	if !strings.Contains(md, "For the Apache Tika project") {
		t.Error("expected markdown output to contain 'For the Apache Tika project'")
	}

	// Check that we have notes (this file has a note "A nice note")
	if !strings.Contains(md, "**Notes:**") {
		t.Error("expected markdown output to contain notes section")
	}

	if !strings.Contains(md, "A nice note") {
		t.Error("expected markdown output to contain 'A nice note'")
	}

	// Check that list items are formatted correctly
	if !strings.Contains(md, "- Some random text for the sake of testability.") {
		t.Error("expected markdown output to contain list item with '- ' prefix")
	}
}
