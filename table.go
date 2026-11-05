package iwaread

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/wmentor/iwaread/internal/base"
	"github.com/wmentor/iwaread/internal/tsp"
)

// Type IDs for table-related objects
const (
	typeTableInfo  = 6000 // TST.TableInfoArchive
	typeTableModel = 6001 // TST.TableModelArchive
	typeTile       = 6002 // TST.Tile
	typeDataList   = 6005 // TST.TableDataList
)

// Cell type constants
const (
	cellTypeEmpty    = 0
	cellTypeSpan     = 1
	cellTypeNumber   = 2
	cellTypeText     = 3
	cellTypeFormula  = 4
	cellTypeDate     = 5
	cellTypeBool     = 6
	cellTypeDuration = 7
	cellTypeError    = 8
	cellTypeRichText = 9
	cellTypeCurrency = 10
)

// Cell flag constants
const (
	cellFlagDecimal128       = 0x1
	cellFlagDouble           = 0x2
	cellFlagSeconds          = 0x4
	cellFlagStringID         = 0x8
	cellFlagRichID           = 0x10
	cellFlagCellStyleID      = 0x20
	cellFlagTextStyleID      = 0x40
	cellFlagCondStyleID      = 0x80
	cellFlagCondRuleStyleID  = 0x100
	cellFlagFormulaID        = 0x200
	cellFlagControlID        = 0x400
	cellFlagFormulaErrorID   = 0x800
	cellFlagSuggestID        = 0x1000
	cellFlagNumFormatID      = 0x2000
	cellFlagCurrencyFormatID = 0x4000
	cellFlagDateFormatID     = 0x8000
	cellFlagDurationFormatID = 0x10000
	cellFlagTextFormatID     = 0x20000
	cellFlagBoolFormatID     = 0x40000
	cellFlagCommentID        = 0x80000
)

// Flag sizes in bytes
var flagSizes = []struct {
	flag uint32
	size int
}{
	{cellFlagDecimal128, 16},
	{cellFlagDouble, 8},
	{cellFlagSeconds, 8},
	{cellFlagStringID, 4},
	{cellFlagRichID, 4},
	{cellFlagCellStyleID, 4},
	{cellFlagTextStyleID, 4},
	{cellFlagCondStyleID, 4},
	{cellFlagCondRuleStyleID, 4},
	{cellFlagFormulaID, 4},
	{cellFlagControlID, 4},
	{cellFlagFormulaErrorID, 4},
	{cellFlagSuggestID, 4},
	{cellFlagNumFormatID, 4},
	{cellFlagCurrencyFormatID, 4},
	{cellFlagDateFormatID, 4},
	{cellFlagDurationFormatID, 4},
	{cellFlagTextFormatID, 4},
	{cellFlagBoolFormatID, 4},
	{cellFlagCommentID, 4},
}

// Table represents a table extracted from a Pages document
type Table struct {
	Rows     int
	Cols     int
	Cells    [][]string
	Position int // Position in the text where the table is embedded
}

// TableAttachment represents a table attachment in the text
type TableAttachment struct {
	Position int    // Position in the text
	TableID  uint64 // Table object ID
}

// extractTables extracts all tables from the document with their positions
func extractTables(store *tsp.ObjectStore, bodyMsg *base.RawMessage) []Table {
	var tables []Table

	// Extract table attachments
	attachments := extractTableAttachments(store, bodyMsg)

	// Extract each table with its position
	for _, attachment := range attachments {
		tableObj := store.Object(attachment.TableID)
		if tableObj == nil {
			continue
		}

		table, err := extractTable(store, tableObj, attachment.Position)
		if err != nil {
			continue
		}

		tables = append(tables, *table)
	}

	return tables
}

// extractTableAttachments extracts table attachments from body storage
func extractTableAttachments(store *tsp.ObjectStore, bodyMsg *base.RawMessage) []TableAttachment {
	var attachments []TableAttachment

	// Get attachment table (field 9)
	attachmentTable, err := bodyMsg.GetMessage(9)
	if err != nil || attachmentTable == nil {
		return attachments
	}

	// Get entries
	entries, err := attachmentTable.GetMessages(1)
	if err != nil {
		return attachments
	}

	// Process each attachment
	for _, entry := range entries {
		// Get character index (field 1)
		charIndex, _ := entry.GetUint(1)

		// Get object reference (field 2)
		objRef, err := entry.GetMessage(2)
		if err != nil || objRef == nil {
			continue
		}

		objID, ok := objRef.GetVarint(1)
		if !ok {
			continue
		}

		obj := store.Object(objID)
		if obj == nil {
			continue
		}

		// Check if it's a DrawableAttachment (type 2003)
		if obj.Type != 2003 {
			continue
		}

		// Get drawable reference (field 1)
		drawableRef, err := obj.Message.GetMessage(1)
		if err != nil || drawableRef == nil {
			continue
		}

		drawableID, ok := drawableRef.GetVarint(1)
		if !ok {
			continue
		}

		drawable := store.Object(drawableID)
		if drawable == nil {
			continue
		}

		// Check if it's a table (type 6000)
		if drawable.Type != typeTableInfo {
			continue
		}

		attachments = append(attachments, TableAttachment{
			Position: charIndex,
			TableID:  drawableID,
		})
	}

	return attachments
}

// extractTable extracts a single table from a TableInfo object
func extractTable(store *tsp.ObjectStore, tableInfo *tsp.IwaObject, position int) (*Table, error) {
	// Get table model reference (field 2)
	modelRef, err := tableInfo.Message.GetMessage(2)
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

	// Get row and column counts
	rows, _ := model.Message.GetUint(6)
	cols, _ := model.Message.GetUint(7)

	// Get data store (field 4)
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

	// Get tiles (field 3 in DataStore)
	tilesMsg, err := dataStoreMsg.GetMessage(3)
	if err != nil || tilesMsg == nil {
		return nil, fmt.Errorf("no tiles message")
	}

	// Get tile references (field 1 in TileStorage - repeated Tile)
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

			// Parse cells
			pos := 0
			for col := 0; col < int(cellCount) && pos < len(cellBuffer); col++ {
				if pos+12 > len(cellBuffer) {
					break
				}

				version := cellBuffer[pos]
				cellType := cellBuffer[pos+1]

				if version != 5 {
					break
				}

				// Read flags (bytes 8-11)
				flags := binary.LittleEndian.Uint32(cellBuffer[pos+8 : pos+12])

				// Calculate data size
				dataSize := 0
				for _, fs := range flagSizes {
					if flags&fs.flag != 0 {
						dataSize += fs.size
					}
				}

				// Extract cell value
				cellValue := extractCellValue(int(cellType), flags, cellBuffer[pos+12:pos+12+dataSize], stringMap)

				if int(rowIndex) < rows && col < cols {
					cells[rowIndex][col] = cellValue
				}

				// Move to next cell
				pos += 12 + dataSize
			}
		}
	}

	return &Table{
		Rows:     rows,
		Cols:     cols,
		Cells:    cells,
		Position: position,
	}, nil
}

// buildStringMap builds a map of string IDs to string values from a string table
func buildStringMap(stringTable *tsp.IwaObject) (map[uint32]string, error) {
	stringMap := make(map[uint32]string)

	// Get entries (field 3)
	entries, err := stringTable.Message.GetMessages(3)
	if err != nil {
		return nil, fmt.Errorf("error getting entries: %w", err)
	}

	for _, entry := range entries {
		// Get key (field 1)
		key, _ := entry.GetUint(1)

		// Get value (field 3) - it's bytes
		valueBytes, ok := entry.GetBytes(3)
		if !ok {
			continue
		}
		stringMap[uint32(key)] = string(valueBytes)
	}

	return stringMap, nil
}

// extractCellValue extracts a cell value from cell data
func extractCellValue(cellType int, flags uint32, data []byte, stringMap map[uint32]string) string {
	switch cellType {
	case cellTypeEmpty:
		return ""
	case cellTypeText:
		// String ID is at offset 0 if STRING_ID flag is set
		if flags&cellFlagStringID != 0 && len(data) >= 4 {
			stringID := binary.LittleEndian.Uint32(data[0:4])
			if str, ok := stringMap[stringID]; ok {
				return str
			}
			return fmt.Sprintf("[string:%d]", stringID)
		}
		return ""
	case cellTypeNumber:
		// Double is at offset 0 if DOUBLE flag is set
		if flags&cellFlagDouble != 0 && len(data) >= 8 {
			bits := binary.LittleEndian.Uint64(data[0:8])
			value := float64frombits(bits)
			return fmt.Sprintf("%g", value)
		}
		return ""
	case cellTypeBool:
		// Bool is stored as varint
		if len(data) >= 1 {
			if data[0] != 0 {
				return "TRUE"
			}
			return "FALSE"
		}
		return ""
	case cellTypeRichText:
		// Rich text requires special handling - for now return placeholder
		return "[rich text]"
	default:
		return fmt.Sprintf("[type:%d]", cellType)
	}
}

// float64frombits converts uint64 bits to float64
func float64frombits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// tableToMarkdown converts a table to markdown format
func tableToMarkdown(table *Table) string {
	if table.Rows == 0 || table.Cols == 0 {
		return ""
	}

	var sb strings.Builder

	// Write header row
	sb.WriteString("| ")
	for col := 0; col < table.Cols; col++ {
		sb.WriteString(table.Cells[0][col])
		sb.WriteString(" | ")
	}
	sb.WriteString("\n")

	// Write separator
	sb.WriteString("|")
	for col := 0; col < table.Cols; col++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Write data rows
	for row := 1; row < table.Rows; row++ {
		sb.WriteString("| ")
		for col := 0; col < table.Cols; col++ {
			sb.WriteString(table.Cells[row][col])
			sb.WriteString(" | ")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
