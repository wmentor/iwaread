# Protobuf Files for Apple iWork

This directory contains protobuf schemas for the Apple iWork file format (.pages, .numbers, .key).

## Main Files

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

### Applications
- `TSCEArchives.proto` - calculation engine (formulas, calculations)
- `TSCHArchives.proto` - charts and diagrams

### Commands
- `*CommandArchives.proto` - editing commands

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

## Notes

1. **Schema-less parser**: The current implementation uses a schema-less protobuf parser (`internal/base/protobuf.go`) that does not require generated code. This allows reading any fields, even unknown ones.

2. **Compatibility**: These schemas are relevant for iWork '13 and newer (2013+). Old iWork '09 formats are not supported.

3. **Apple does not publish schemas**: These protobuf files were obtained through reverse engineering and are not official Apple documentation.

## Object Types

### Pages (type IDs)
- `10000` - TP.DocumentArchive (root object)
- `10001` - TP.ThemeArchive
- `10011` - TP.SectionArchive
- `10012` - TP.SettingsArchive

### Common (TSWP)
- `2001` - TSWP.StorageArchive (text storage)
- `2021` - TSWP.CharacterStyleArchive
- `2022` - TSWP.ParagraphStyleArchive
- `2023` - TSWP.ListStyleArchive
- `2032` - TSWP.HyperlinkFieldArchive

### Tables (TST)
- `6000` - TST.TableInfoArchive
- `6001` - TST.TableModelArchive
- `6002` - TST.Tile

### Graphics (TSD)
- `3002` - TSD.DrawableArchive
- `3005` - TSD.ImageArchive
- `3004` - TSD.ShapeArchive
