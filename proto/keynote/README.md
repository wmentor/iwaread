# Keynote Protocol Buffer Definitions

This directory contains Protocol Buffer (.proto) files for parsing Apple Keynote files.

## Files

### Keynote-specific files:
- `KNArchives.proto` - Main Keynote archive definitions
- `KNArchives_sos.proto` - Keynote archive SOS (Save/Open/Save) definitions
- `KNCommandArchives.proto` - Keynote command archive definitions
- `KNCommandArchives_sos.proto` - Keynote command archive SOS definitions

### Common iWork files (shared with Pages and Numbers):
- `TSPMessages.proto` - Base TSP (iWork) message definitions
- `TSPArchiveMessages.proto` - TSP archive message definitions
- `TSPDatabaseMessages.proto` - TSP database message definitions
- `TSKArchives.proto` - Common iWork archive definitions
- `TSDArchives.proto` - Drawing archives definitions
- `TSDCommandArchives.proto` - Drawing command archives
- `TSWPArchives.proto` - Text processing archives definitions
- `TSWPCommandArchives.proto` - Text processing command archives
- `TSSArchives.proto` - Style archives definitions
- `TSTArchives.proto` - Table archives definitions
- `TSTCommandArchives.proto` - Table command archives
- `TSTStylePropertyArchiving.proto` - Table style property archiving
- `TSAArchives.proto` - Common archive definitions

## Usage

These proto files can be used to generate code for parsing Keynote files:

```bash
protoc --go_out=. *.proto
```

## Source

These files were extracted from Apple's iWork applications and represent the internal data structures used in Keynote files (.key).
