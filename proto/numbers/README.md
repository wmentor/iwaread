# Protobuf Files for Apple Numbers

This directory contains protobuf schemas for the Apple Numbers file format (.numbers).

## Numbers-Specific Files

### TN (Numbers Application)
- `TNArchives.proto` - Numbers document structure (DocumentArchive, SheetArchive)
- `TNCommandArchives.proto` - Numbers-specific editing commands

## Common Files (Shared with Pages/Keynote)

### Basic Types
- `TSPMessages.proto` - basic types (Reference, Range, Color, Point, Size)
- `TSPArchiveMessages.proto` - archive metadata
- `TSPDatabaseMessages.proto` - database messages

### Styles and Themes
- `TSSArchives.proto` - styles and stylesheets (StyleArchive, StylesheetArchive, ThemeArchive)

### Text and Documents
- `TSKArchives.proto` - basic document types (DocumentArchive, CommandHistory)
- `TSAArchives.proto` - common application types (DocumentArchive for Numbers/Pages/Keynote)
- `TSWPArchives.proto` - text storages (StorageArchive, ParagraphStyleArchive, CharacterStyleArchive)

### Graphics and Images
- `TSDArchives.proto` - graphic objects (DrawableArchive, ImageArchive, ShapeArchive)

### Tables
- `TSTArchives.proto` - tables (TableInfoArchive, TableModelArchive, DataStore, Tile)
- `TSTStylePropertyArchiving.proto` - table style properties

### Calculation Engine
- `TSCEArchives.proto` - calculation engine (formulas, calculations, cell dependencies)

### Charts
- `TSCHArchives.proto` - charts and diagrams
- `TSCHArchives_Common.proto` - common chart types
- `TSCHArchives_GEN.proto` - generated chart definitions
- `TSCH3DArchives.proto` - 3D chart rendering

### Commands
- `TSTCommandArchives.proto` - table editing commands
- `TSDCommandArchives.proto` - drawing editing commands
- `TSWPCommandArchives.proto` - text editing commands
- `TSCHCommandArchives.proto` - chart editing commands
- `TNCommandArchives.proto` - Numbers-specific commands

## Usage

### Schema-less Parser (Recommended)

The current implementation uses a schema-less protobuf parser (`internal/base/protobuf.go`) that does not require generated code. This allows:
- Reading any fields, even unknown ones
- Preserving unknown fields unchanged
- Working without dependencies on protobuf libraries

```go
// Example of using schema-less parser
msg, err := base.ParseMessage(data)
value, ok := msg.GetVarint(1) // Get field 1 as varint
```

### Reading Numbers Files

```go
// Read a Numbers file
content, err := ReadNumbers("file.numbers")
if err != nil {
    log.Fatal(err)
}
fmt.Println(content) // Markdown output with tables
```

### Key Numbers Structures

1. **TN.DocumentArchive** (type 1): Root object containing sheets
2. **TN.SheetArchive** (type 2): Contains table references
3. **TST.TableModelArchive** (type 6001): Table structure with data store
4. **TST.DataStore**: Contains tiles (field 3), string table (field 4), format table (field 22)
5. **TST.Tile**: Contains row infos with cell data
6. **Cell Record**: 12-byte header + variable data (decimal128, double, string ID, format ID, etc.)

## Notes

1. **Schema-less parser**: The current implementation uses a schema-less protobuf parser (`internal/base/protobuf.go`) that does not require generated code. This allows reading any fields, even unknown ones.

2. **Compatibility**: These schemas are relevant for iWork '13 and newer (2013+). Old iWork '09 formats are not supported.

3. **Apple does not publish schemas**: These protobuf files were obtained through reverse engineering and are not official Apple documentation.

4. **Numbers-specific features**: Numbers uses the calculation engine (TSCE) for formulas and cell dependencies. The format table (field 22 in TST.DataStore) stores cell formatting information (number, currency, percentage, date formats).

5. **Decimal128**: Numbers stores numeric values in IEEE 754-2008 decimal128 format (16 bytes) with a 113-bit significand and biased exponent (bias 6176).

## Object Types

### Numbers (type IDs)
- `1` - TN.DocumentArchive (root object)
- `2` - TN.SheetArchive

### Tables (TST)
- `6000` - TST.TableInfoArchive
- `6001` - TST.TableModelArchive
- `6002` - TST.Tile
- `6005` - TST.TableDataList (string table, format table)

### Calculation Engine (TSCE)
- `13000` - TSCE.CalculationEngineArchive
- `13001` - TSCE.FormulaOwnerDependenciesArchive

### Common (TSWP)
- `2001` - TSWP.StorageArchive (text storage)
- `2021` - TSWP.CharacterStyleArchive
- `2022` - TSWP.ParagraphStyleArchive
- `2023` - TSWP.ListStyleArchive
- `2032` - TSWP.HyperlinkFieldArchive

### Graphics (TSD)
- `3002` - TSD.DrawableArchive
- `3005` - TSD.ImageArchive
- `3004` - TSD.ShapeArchive

### Charts (TSCH)
- `4000` - TSCH.ChartArchive
- `4001` - TSCH.ChartModelArchive
