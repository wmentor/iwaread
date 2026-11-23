<img src="./docs/images/logo.jpg" width="400px;" style="border-radius:10px;">

<br/>

Go library for reading Apple iWork files (.pages, .numbers, .key) and converting them to markdown.

## Summary

- Require Go version >= 1.26
- Written on pure Go

## Features

- Read `.pages` files and convert to markdown
- Read `.numbers` files and convert to markdown (each table as H1 heading)
- Read `.key` files and convert to markdown (slides with titles, body text, and speaker notes)
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

### Reading .pages files

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

### Reading .numbers files

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/wmentor/iwaread"
)

func main() {
    markdown, err := iwaread.ReadNumbers("spreadsheet.numbers")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(markdown)
}
```

Each table in the Numbers document is rendered as an H1 heading with the table name, followed by the table in markdown format.

### Reading .key files

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/wmentor/iwaread"
)

func main() {
    markdown, err := iwaread.ReadKeynote("presentation.key")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(markdown)
}
```

Each slide in the Keynote presentation is rendered with a heading (slide number and title if available), followed by body text and speaker notes (if any).

## How It Works

Apple iWork files (.pages, .numbers, .key) are ZIP archives containing:
- IWA files (Snappy-compressed protobuf messages)
- Metadata and Data directories

The library:
1. Unpacks the ZIP archive
2. Decodes IWA files (Snappy decompression)
3. Parses protobuf messages (schema-less parser)
4. Builds an object graph
5. Extracts content (text for Pages, tables for Numbers, slides for Keynote)
6. Converts to markdown

### Pages Documents

For .pages files, the library:
- Extracts text from body storage
- Converts to markdown using universal style identifiers

### Numbers Documents

For .numbers files, the library:
- Iterates through sheets and their tables
- Extracts cell data from tile storage
- Renders each table as an H1 heading with the table name, followed by the table in markdown format

### Keynote Presentations

For .key files, the library:
- Traverses the slide tree in presentation order
- Extracts title and body text from placeholders
- Extracts speaker notes from note archives
- Renders each slide with its number, title, body text, and notes

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
- Some cell types (dates, currency, durations) may show as placeholders

## License

MIT
