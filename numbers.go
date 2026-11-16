package iwaread

import (
	"fmt"
	"os"
	"strings"

	"github.com/wmentor/iwaread/internal/tsp"
)

// Type IDs for Numbers-specific objects.
const (
	typeTNDocument = 1 // TN.DocumentArchive.
	typeTNSheet    = 2 // TN.SheetArchive.
	// typeTableInfo = 6000 is declared in table.go.
)

// TN.DocumentArchive field numbers.
const (
	tnDocSheets = 1
)

// TN.SheetArchive field numbers.
const (
	tnSheetName          = 1
	tnSheetDrawableInfos = 2
)

// TST.TableInfoArchive field numbers.
const (
	tableInfoTableModel = 2
)

// TST.TableModelArchive field numbers.
const (
	tableModelTableName = 8
)

// ReadNumbers reads a .numbers file and returns its content as markdown.
// Each table is rendered as an H1 heading with the table name, followed by
// the table in markdown format.
func ReadNumbers(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	container, err := tsp.IWorkContainerFromBytes(data)
	if err != nil {
		return "", err
	}

	store, err := tsp.NewObjectStore(container)
	if err != nil {
		return "", err
	}

	return extractNumbersMarkdown(store)
}

func extractNumbersMarkdown(store *tsp.ObjectStore) (string, error) {
	// Find TN.DocumentArchive
	docObj := store.FindByType(typeTNDocument)
	if docObj == nil {
		return "", fmt.Errorf("TN.DocumentArchive not found — not a Numbers document?")
	}

	// Get sheets
	sheetRefs, err := docObj.Message.GetMessages(tnDocSheets)
	if err != nil {
		return "", fmt.Errorf("failed to get sheets: %w", err)
	}

	var sb strings.Builder

	for _, sheetRef := range sheetRefs {
		// Resolve sheet reference
		sheetID, ok := sheetRef.GetVarint(1)
		if !ok {
			continue
		}

		sheetObj := store.Object(sheetID)
		if sheetObj == nil || sheetObj.Type != typeTNSheet {
			continue
		}

		// Get sheet name
		sheetName, _ := sheetObj.Message.GetString(tnSheetName)

		// Get drawable infos
		drawableRefs, err := sheetObj.Message.GetMessages(tnSheetDrawableInfos)
		if err != nil {
			continue
		}

		for _, drawableRef := range drawableRefs {
			// Resolve drawable reference
			drawableID, ok := drawableRef.GetVarint(1)
			if !ok {
				continue
			}

			drawableObj := store.Object(drawableID)
			if drawableObj == nil {
				continue
			}

			// Check if it's a table
			if drawableObj.Type != typeTableInfo {
				continue
			}

			// Extract table
			table, err := extractNumbersTable(store, drawableObj)
			if err != nil {
				continue
			}

			// Write table name as H1
			tableName := table.Name
			if tableName == "" {
				tableName = sheetName
			}
			if tableName != "" {
				sb.WriteString("# ")
				sb.WriteString(tableName)
				sb.WriteString("\n\n")
			}

			// Write table as markdown
			sb.WriteString(numbersTableToMarkdown(table))
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// NumbersTable represents a table extracted from a Numbers document.
type NumbersTable struct {
	Name     string
	Rows     int
	Cols     int
	Cells    [][]string
	Position int // Not used for Numbers, kept for compatibility
}

// extractNumbersTable extracts a single table from a TableInfo object.
func extractNumbersTable(store *tsp.ObjectStore, tableInfo *tsp.IwaObject) (*NumbersTable, error) {
	// Get table model reference (field 2)
	modelRef, err := tableInfo.Message.GetMessage(tableInfoTableModel)
	if err != nil || modelRef == nil {
		return nil, fmt.Errorf("no table model reference")
	}

	modelID, ok := modelRef.GetVarint(1)
	if !ok {
		return nil, fmt.Errorf("could not get model ID")
	}

	model := store.Object(modelID)
	if model == nil {
		return nil, fmt.Errorf("model object not found")
	}

	// Get table name (field 8)
	tableName, _ := model.Message.GetString(tableModelTableName)

	// Get row and column counts.
	rows, _ := model.Message.GetUint(6)
	cols, _ := model.Message.GetUint(7)

	// Get data store (field 4).
	dataStoreMsg, err := model.Message.GetMessage(4)
	if err != nil || dataStoreMsg == nil {
		return nil, fmt.Errorf("no data store message")
	}

	// Get string table (field 4 in DataStore)
	stringTableRef, err := dataStoreMsg.GetMessage(4)
	if err != nil || stringTableRef == nil {
		return nil, fmt.Errorf("no string table reference")
	}

	stringTableID, ok := stringTableRef.GetVarint(1)
	if !ok {
		return nil, fmt.Errorf("could not get string table ID")
	}

	stringTable := store.Object(stringTableID)
	if stringTable == nil {
		return nil, fmt.Errorf("string table object not found")
	}

	// Build string map
	stringMap, err := buildStringMap(stringTable)
	if err != nil {
		return nil, fmt.Errorf("failed to build string map: %w", err)
	}

	// Build rich text map (field 17 in DataStore)
	richTextMap := buildRichTextMap(store, dataStoreMsg)

	// Build format map (field 11 in DataStore)
	formatMap := buildFormatMap(store, dataStoreMsg)

	// Get tiles (field 3 in DataStore)
	tilesMsg, err := dataStoreMsg.GetMessage(3)
	if err != nil || tilesMsg == nil {
		return nil, fmt.Errorf("no tiles message")
	}

	// Get tile references (field 1 in TileStorage - repeated Tile).
	tileRefs, err := tilesMsg.GetMessages(1)
	if err != nil {
		return nil, fmt.Errorf("error getting tile refs: %w", err)
	}

	// Initialize cells array
	cells := make([][]string, rows)
	for i := range cells {
		cells[i] = make([]string, cols)
	}

	// Process each tile
	for _, tileRef := range tileRefs {
		// Get tile reference (field 2)
		tileObjRef, err := tileRef.GetMessage(2)
		if err != nil || tileObjRef == nil {
			continue
		}

		tileObjID, ok := tileObjRef.GetVarint(1)
		if !ok {
			continue
		}

		tileObj := store.Object(tileObjID)
		if tileObj == nil {
			continue
		}

		// Get row infos (field 5 in Tile)
		rowInfos, err := tileObj.Message.GetMessages(5)
		if err != nil {
			continue
		}

		// Process each row
		for _, rowInfo := range rowInfos {
			// Get row index (field 1)
			rowIndex, _ := rowInfo.GetUint(1)

			// Get cell count (field 2)
			cellCount, _ := rowInfo.GetUint(2)

			// Get cell storage buffer (field 6)
			cellBuffer, ok := rowInfo.GetBytes(6)
			if !ok {
				continue
			}

			// Parse cells sequentially
			parseCellsSequentially(cellBuffer, cellCount, rowIndex, rows, cols, cells, stringMap, richTextMap, formatMap)
		}
	}

	return &NumbersTable{
		Name:  tableName,
		Rows:  rows,
		Cols:  cols,
		Cells: cells,
	}, nil
}

// readLittleEndianUint32 reads a uint32 from bytes in little-endian order.
func readLittleEndianUint32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// parseCellsSequentially parses cells sequentially without using offsets (fallback).
func parseCellsSequentially(
	cellBuffer []byte,
	cellCount, rowIndex, rows, cols int,
	cells [][]string,
	stringMap map[uint32]string,
	richTextMap map[uint32]string,
	formatMap map[uint32]*cellFormat,
) {
	pos := 0
	for col := 0; col < cellCount && pos < len(cellBuffer); col++ {
		if pos+12 > len(cellBuffer) {
			break
		}

		version := cellBuffer[pos]
		cellType := cellBuffer[pos+1]

		if version != 5 {
			break
		}

		// Read flags (bytes 8-11)
		flags := readLittleEndianUint32(cellBuffer[pos+8 : pos+12])

		// Calculate data size
		dataSize := 0
		for _, fs := range flagSizes {
			if flags&fs.flag != 0 {
				dataSize += fs.size
			}
		}

		if pos+12+dataSize > len(cellBuffer) {
			break
		}

		// Extract cell value
		cellValue := extractCellValue(
			int(cellType), flags, cellBuffer[pos+12:pos+12+dataSize], stringMap, richTextMap, formatMap,
		)

		if rowIndex < rows && col < cols {
			cells[rowIndex][col] = cellValue
		}

		// Move to next cell
		pos += 12 + dataSize
	}
}

// numbersTableToMarkdown converts a NumbersTable to markdown format.
func numbersTableToMarkdown(table *NumbersTable) string {
	return renderMarkdownTable(table.Rows, table.Cols, table.Cells)
}
