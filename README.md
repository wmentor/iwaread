# iwaread

Go library for reading Apple iWork files (.pages) and converting them to markdown.

## Summary

- Require Go version >= 1.26
- Written on pure Go

## Features

- Read `.pages` files and convert to markdown
- Support for headings (Heading 1-6 → #, ##, ###, ...)
- Support for lists (bulleted → -)
- Support for hyperlinks
- Support for bold and italic text
- Support for tables (extracted and converted to markdown tables)

## Installation

```bash
go get github.com/wmentor/iwaread
```

## Development

### Protobuf Schemas

The project contains Apple iWork protobuf schemas in the `proto/` directory. These files are taken from the cupertino-files project and serve as format documentation.

**Important**: The current implementation uses a schema-less protobuf parser that does not require generated code. This allows:
- Reading any fields, even unknown ones
- Preserving unknown fields unchanged
- Working without dependencies on protobuf libraries

## Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/wmentor/iwaread"
)

func main() {
    markdown, err := iwaread.ReadPages("document.pages")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(markdown)
}
```

## How It Works

Apple iWork files (.pages) are ZIP archives containing:
- IWA files (Snappy-compressed protobuf messages)
- Metadata and Data directories

The library:
1. Unpacks the ZIP archive
2. Decodes IWA files (Snappy decompression)
3. Parses protobuf messages (schema-less parser)
4. Builds an object graph
5. Extracts text from body storage
6. Converts to markdown using universal style identifiers

### Universal Style Detection

The library uses **style identifiers** (not localized names) to detect headings and lists:
- **Headings**: identifier contains `paragraphstyle-title` or `paragraphstyle-heading`
- **Lists**: list style exists AND identifier does NOT contain `None` (identifier can be empty for dynamic list styles)
- **Heading levels**: extracted from identifier (e.g., `Heading 2` → level 2)

This approach works regardless of document language (English, Russian, etc.).

## Limitations

- Only modern iWork files are supported (iWork '13 and newer)
- Old iWork '09 XML files are not supported
- Password-protected documents are not supported
- Images are not extracted (text only)
- Tables are not converted to markdown tables

## License

MIT
