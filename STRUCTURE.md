# iwaread Project Structure

## Architecture

The project ports the .pages file reading logic from the TypeScript cupertino-files library to Go.

### Modules

#### internal/base/
Basic primitives for working with binary data:
- `varint.go` - varint encoding/decoding (protobuf)
- `protobuf.go` - schema-less protobuf parser (RawMessage)
- `snappy.go` - Snappy decompression for IWA files

#### internal/tsp/
iWork package format components:
- `package.go` - IWorkContainer (ZIP archive)
- `iwa.go` - IWA stream parser (object archives)
- `store.go` - ObjectStore (object graph by ID)

#### Root Package
- `iwaread.go` - main ReadPages function and markdown conversion logic
- `table.go` - table extraction from iWork documents and conversion to markdown
- `iwaread_test.go` - tests

#### cmd/iwaread/
- `main.go` - CLI utility for converting .pages to markdown

#### proto/
Protobuf schemas for Apple iWork format:
- `TSPMessages.proto` - basic types (Reference, Range, Color)
- `TSWPArchives.proto` - text storages
- `TSTArchives.proto` - tables
- `TSDArchives.proto` - graphics and images
- `TSKArchives.proto` - basic document types
- and others...

Files are taken from cupertino-files/proto/current/.

#### Makefile
Commands for development:
- `make unit/test` - run unit-tests

## How It Works

1. **ZIP Unpacking** (internal/tsp/package.go)
   - Uses the standard library archive/zip
   - Support for flat and nested layouts
   - Extracting .iwa files

2. **Snappy Decompression** (internal/base/snappy.go)
   - Apple uses its own Snappy chunks format
   - Each chunk: type (1 byte) + length (3 bytes) + data
   - Decompression of each chunk and concatenation

3. **IWA Parsing** (internal/tsp/iwa.go)
   - Format: varint(length) + ArchiveInfo + payloads
   - ArchiveInfo contains identifier and message_infos
   - Each payload is a protobuf message

4. **Protobuf Parsing** (internal/base/protobuf.go)
   - Schema-less parser (does not require .proto files)
   - Support for all wire types (varint, fixed32, fixed64, bytes)
   - RawMessage stores fields and allows extracting values

5. **Object Store** (internal/tsp/store.go)
   - Indexes all objects by identifier
   - Allows finding objects by type
   - Resolves TSP.Reference links

6. **Text Extraction** (iwaread.go)
   - Finds TP.DocumentArchive (type 10000)
   - Gets body_storage reference
   - Extracts text from StorageArchive (field 3)
   - Reads style tables (para_style, char_style, list_style, smartfield)

7. **Markdown Conversion** (iwaread.go)
   - Splits text into paragraphs
   - Uses universal style identifiers (not localized names) to detect headings and lists
   - Determines headings by identifier containing "paragraphstyle-title" or "paragraphstyle-heading"
   - Determines lists by checking if list style exists AND identifier does NOT contain "None" (identifier can be empty for dynamic list styles)
   - Extracts heading levels from identifiers (e.g., "Heading 2" → level 2)
   - Applies bold/italic by char_style
   - Extracts hyperlinks from smartfield table
   - Extracts tables and inserts them at correct positions in markdown

## Key iWork Types

- TP.DocumentArchive (10000) - root object of Pages document
- TSWP.StorageArchive (2001) - text storage
- TSWP.ParagraphStyleArchive (2022) - paragraph style
- TSWP.CharacterStyleArchive (2021) - character style
- TSWP.ListStyleArchive (2023) - list style
- TSWP.HyperlinkFieldArchive (2032) - hyperlink
- TST.TableInfoArchive (6000) - table information
- TST.TableModelArchive (6001) - table model with data
- TST.Tile (6002) - table tile with cell data
- TST.TableDataList (6005) - string table for cell values

## Key Fields

### TP.DocumentArchive
- field 4: body_storage (reference to TSWP.StorageArchive)

### TSWP.StorageArchive
- field 1: kind (storage type)
- field 3: text (text)
- field 5: table_para_style (paragraph style table)
- field 7: table_list_style (list style table)
- field 8: table_char_style (character style table)
- field 9: table_attachment (attachment table with table references)
- field 11: table_smartfield (smart field table, including hyperlinks)

### TST.TableInfoArchive
- field 2: table_model (reference to TST.TableModelArchive)

### TST.TableModelArchive
- field 4: base_data_store (reference to TST.DataStore)
- field 6: number_of_rows
- field 7: number_of_columns
- field 8: table_name

### TST.DataStore
- field 3: tiles (reference to TST.TileStorage)
- field 4: string_table (reference to TST.TableDataList)

### TST.TileStorage
- field 1: tiles (repeated TST.Tile references)

### TST.Tile
- field 5: row_infos (repeated TST.TileRowInfo)

### TST.TileRowInfo
- field 1: row_index
- field 2: cell_count
- field 6: cell_storage_buffer (binary cell data)

### ObjectAttributeTable
- field 1: entries (array of ObjectAttribute)
- ObjectAttribute:
  - field 1: character_index (position in text)
  - field 2: object (reference to style)

## Limitations

- Read-only (no editing)
- Only .pages (no .numbers and .key)
- No image support
- Rich text cells in tables show as "[rich text]" (plain text cells fully supported)
- No password-protected document support
- Only modern iWork files (iWork '13+)

## Testing

```bash
go test ./...
```

Tests use fixture files from cupertino-files/fixtures/.

## CLI

```bash
go run cmd/iwaread/main.go document.pages
```

## Dependencies

Only the Go standard library:
- archive/zip
- encoding/binary
- fmt
- strings
- os
