package tsp

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// IWorkContainer represents an iWork package (ZIP archive).
type IWorkContainer struct {
	IwaFiles map[string][]byte
}

// IWorkContainerFromBytes parses an iWork package from bytes.
func IWorkContainerFromBytes(data []byte) (*IWorkContainer, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}

	c := &IWorkContainer{
		IwaFiles: make(map[string][]byte),
	}

	// Detect wrapper directory prefix
	var prefix string
	var fileNames []string
	for _, f := range r.File {
		if !f.FileInfo().IsDir() {
			fileNames = append(fileNames, f.Name)
		}
	}
	if len(fileNames) > 0 {
		first := fileNames[0]
		slashIdx := strings.Index(first, "/")
		if slashIdx > 0 {
			candidate := first[:slashIdx+1]
			allMatch := true
			for _, n := range fileNames {
				if !strings.HasPrefix(n, candidate) {
					allMatch = false
					break
				}
			}
			if allMatch {
				prefix = candidate
			}
		}
	}

	// Check for nested Index.zip
	var indexZipData []byte
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}
		if strings.HasSuffix(strings.ToLower(name), "index.zip") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			indexZipData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}

	if indexZipData != nil {
		// Nested layout
		innerR, err := zip.NewReader(bytes.NewReader(indexZipData), int64(len(indexZipData)))
		if err != nil {
			return nil, fmt.Errorf("inner zip: %w", err)
		}
		for _, f := range innerR.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if strings.HasSuffix(f.Name, ".iwa") {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return nil, err
				}
				c.IwaFiles[canonicalIwaName(f.Name)] = data
			}
		}
	} else {
		// Flat layout
		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			name := f.Name
			if prefix != "" {
				name = strings.TrimPrefix(name, prefix)
			}
			if strings.HasSuffix(name, ".iwa") {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return nil, err
				}
				c.IwaFiles[canonicalIwaName(name)] = data
			}
		}
	}

	if len(c.IwaFiles) == 0 {
		return nil, fmt.Errorf("no .iwa components found — not a modern iWork file")
	}

	return c, nil
}

// canonicalIwaName normalizes an IWA path to "Index/<...>.iwa" form.
func canonicalIwaName(name string) string {
	n := name
	for strings.HasPrefix(n, "/") {
		n = n[1:]
	}
	if !strings.HasPrefix(n, "Index/") {
		idx := strings.Index(n, "Index/")
		if idx >= 0 {
			n = n[idx:]
		} else {
			n = "Index/" + n
		}
	}
	return n
}
