package iwaread

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/wmentor/iwaread/internal/base"
	"github.com/wmentor/iwaread/internal/tsp"
)

// Type IDs for table-related objects.
const (
	typeTableInfo  = 6000 // TST.TableInfoArchive
	typeTableModel = 6001 // TST.TableModelArchive
	typeTile       = 6002 // TST.Tile
	typeDataList   = 6005 // TST.TableDataList
)

// Cell type constants.
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

// Cell flag constants.
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

// Format type codes (TSK.FormatStructArchive.format_type).
const (
	formatTypeBoolean        = 1
	formatTypeNumber         = 256
	formatTypeCurrency       = 257
	formatTypePercentage     = 258
	formatTypeText           = 260
	formatTypeDate           = 261
	formatTypeCustomNumber   = 270
	formatTypeCustomText     = 271
	formatTypeCustomDate     = 272
	formatTypeCustomDur      = 273
	formatTypeCustomCurrency = 274
)

// cellFormat describes how a cell value is displayed.
type cellFormat struct {
	kind               int // formatType* constant
	decimalPlaces      int // number of decimal places; -1 means "auto"
	thousandsSeparator bool
	currencyCode       string
}

// Flag sizes in bytes.
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

// Table represents a table extracted from a Pages document.
type Table struct {
	Rows     int
	Cols     int
	Cells    [][]string
	Position int // Position in the text where the table is embedded
}

// TableAttachment represents a table attachment in the text.
type TableAttachment struct {
	Position int    // Position in the text
	TableID  uint64 // Table object ID
}

// extractTables extracts all tables from the document with their positions.
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

// extractTableAttachments extracts table attachments from body storage.
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

		// Check if it's a DrawableAttachment (type 2003).
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

// extractTable extracts a single table from a TableInfo object.
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

			// Parse cells
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
				flags := binary.LittleEndian.Uint32(cellBuffer[pos+8 : pos+12])

				// Calculate data size
				dataSize := 0
				for _, fs := range flagSizes {
					if flags&fs.flag != 0 {
						dataSize += fs.size
					}
				}

				// Extract cell value
				cellValue := extractCellValue(int(cellType), flags, cellBuffer[pos+12:pos+12+dataSize], stringMap, richTextMap, formatMap)

				if rowIndex < rows && col < cols {
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

// buildStringMap builds a map of string IDs to string values from a string table.
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

// buildRichTextMap builds a map of rich text IDs to text strings from the rich text table.
func buildRichTextMap(store *tsp.ObjectStore, dataStoreMsg *base.RawMessage) map[uint32]string {
	richTextMap := make(map[uint32]string)

	// Get rich text table reference (field 17 in DataStore)
	richTextTableRef, err := dataStoreMsg.GetMessage(17)
	if err != nil || richTextTableRef == nil {
		return richTextMap
	}

	richTextTableID, ok := richTextTableRef.GetVarint(1)
	if !ok {
		return richTextMap
	}

	richTextTable := store.Object(richTextTableID)
	if richTextTable == nil {
		return richTextMap
	}

	// Get entries (field 3)
	entries, err := richTextTable.Message.GetMessages(3)
	if err != nil {
		return richTextMap
	}

	for _, entry := range entries {
		// Get key (field 1)
		key, _ := entry.GetUint(1)

		// Get rich text payload reference (field 9)
		rtRef, err := entry.GetMessage(9)
		if err != nil || rtRef == nil {
			continue
		}

		rtID, ok := rtRef.GetVarint(1)
		if !ok {
			continue
		}

		// Get the rich text payload object
		rtObj := store.Object(rtID)
		if rtObj == nil {
			continue
		}

		// Get storage reference (field 1 in RichTextPayloadArchive).
		storageRef, err := rtObj.Message.GetMessage(1)
		if err != nil || storageRef == nil {
			continue
		}

		storageID, ok := storageRef.GetVarint(1)
		if !ok {
			continue
		}

		// Get the storage object
		storageObj := store.Object(storageID)
		if storageObj == nil {
			continue
		}

		// Get text from storage (field 3 in StorageArchive).
		texts := storageObj.Message.GetStrings(3)
		if len(texts) > 0 {
			richTextMap[uint32(key)] = texts[0]
		}
	}

	return richTextMap
}

// buildFormatMap builds a map of format IDs to cellFormat structs from the format table.
// The format table is field 22 in DataStore (field 11 is format_table_pre_bnc for older files).
func buildFormatMap(store *tsp.ObjectStore, dataStoreMsg *base.RawMessage) map[uint32]*cellFormat {
	formatMap := make(map[uint32]*cellFormat)

	// Get format table reference (field 22 in DataStore)
	formatTableRef, err := dataStoreMsg.GetMessage(22)
	if err != nil || formatTableRef == nil {
		// Try field 11 for pre-BNC files
		formatTableRef, err = dataStoreMsg.GetMessage(11)
		if err != nil || formatTableRef == nil {
			return formatMap
		}
	}

	formatTableID, ok := formatTableRef.GetVarint(1)
	if !ok {
		return formatMap
	}

	formatTable := store.Object(formatTableID)
	if formatTable == nil {
		return formatMap
	}

	// Get entries (field 3)
	entries, err := formatTable.Message.GetMessages(3)
	if err != nil {
		return formatMap
	}

	for _, entry := range entries {
		// Get key (field 1)
		key, _ := entry.GetUint(1)

		// Get format message (field 6) - this is inline, not a reference
		fmtMsg, err := entry.GetMessage(6)
		if err != nil || fmtMsg == nil {
			continue
		}

		// Parse TSK.FormatStructArchive directly from the inline message
		cf := parseFormat(fmtMsg)
		if cf != nil {
			formatMap[uint32(key)] = cf
		}
	}

	return formatMap
}

// parseFormat parses a TSK.FormatStructArchive into a cellFormat.
func parseFormat(msg *base.RawMessage) *cellFormat {
	// format_type is field 1
	formatType, ok := msg.GetUint(1)
	if !ok {
		return nil
	}

	cf := &cellFormat{
		kind:          formatType,
		decimalPlaces: -1, // auto
	}

	// decimal_places is field 2
	if dp, ok := msg.GetUint(2); ok {
		if dp == 253 {
			cf.decimalPlaces = -1 // auto
		} else {
			cf.decimalPlaces = dp
		}
	}

	// show_thousands_separator is field 5 (bool)
	if sep, ok := msg.GetBool(5); ok {
		cf.thousandsSeparator = sep
	}

	// currency_code is field 3 (string)
	if code, ok := msg.GetString(3); ok {
		cf.currencyCode = code
	}

	return cf
}

// getCellFormat returns the format for a cell based on its flags and data.
func getCellFormat(flags uint32, data []byte, formatMap map[uint32]*cellFormat) *cellFormat {
	// Walk through flags in order to find the format ID in the data section
	offset := 0
	for _, fs := range flagSizes {
		if flags&fs.flag == 0 {
			continue
		}
		// Check if this is a format flag
		switch fs.flag {
		case cellFlagNumFormatID, cellFlagCurrencyFormatID, cellFlagDateFormatID,
			cellFlagDurationFormatID, cellFlagTextFormatID, cellFlagBoolFormatID:
			if offset+4 <= len(data) {
				formatID := binary.LittleEndian.Uint32(data[offset : offset+4])
				if cf, ok := formatMap[formatID]; ok {
					return cf
				}
			}
			return nil
		}
		offset += fs.size
	}
	return nil
}

// formatFloat formats a float64 according to a cellFormat.
func formatFloat(value float64, cf *cellFormat) string {
	if cf == nil {
		return formatFloatAuto(value)
	}

	switch cf.kind {
	case formatTypePercentage:
		displayValue := value * 100
		dp := cf.decimalPlaces
		if dp < 0 {
			dp = autoDecimals(displayValue)
		}
		return fmt.Sprintf("%s%%", formatFixed(displayValue, dp, cf.thousandsSeparator))

	case formatTypeCurrency:
		dp := cf.decimalPlaces
		if dp < 0 {
			dp = 2
		}
		symbol := "$" //nolint:ineffassign // skip.
		switch cf.currencyCode {
		case "EUR":
			symbol = "€"
		case "GBP":
			symbol = "£"
		case "JPY":
			symbol = "¥"
		case "USD", "":
			symbol = "$"
		default:
			symbol = cf.currencyCode + " "
		}
		return fmt.Sprintf("%s%s", symbol, formatFixed(value, dp, cf.thousandsSeparator))

	case formatTypeNumber:
		dp := cf.decimalPlaces
		if dp < 0 {
			dp = autoDecimals(value)
		}
		return formatFixed(value, dp, cf.thousandsSeparator)

	default:
		return formatFloatAuto(value)
	}
}

// formatFloatAuto formats a float with automatic precision (like %g but cleaner).
func formatFloatAuto(value float64) string {
	if value == 0 {
		return "0"
	}
	// Round to avoid floating point artifacts like 143.79999999999998
	rounded := math.Round(value*1e10) / 1e10
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

// formatFixed formats a float with a fixed number of decimal places.
func formatFixed(value float64, decimals int, thousands bool) string {
	result := strconv.FormatFloat(value, 'f', decimals, 64)
	if !thousands {
		return result
	}
	// Insert thousands separators
	dotIdx := strings.Index(result, ".")
	intPart := result
	fracPart := ""
	if dotIdx >= 0 {
		intPart = result[:dotIdx]
		fracPart = result[dotIdx:]
	}
	// Add commas to integer part
	negative := false
	if strings.HasPrefix(intPart, "-") {
		negative = true
		intPart = intPart[1:]
	}
	if len(intPart) > 3 {
		var buf strings.Builder
		first := len(intPart) % 3
		if first > 0 {
			buf.WriteString(intPart[:first])
		}
		for i := first; i < len(intPart); i += 3 {
			if buf.Len() > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(intPart[i : i+3])
		}
		intPart = buf.String()
	}
	if negative {
		intPart = "-" + intPart
	}
	return intPart + fracPart
}

// autoDecimals returns the number of decimal places needed to represent a value.
func autoDecimals(value float64) int {
	if value == 0 {
		return 0
	}
	// Round to avoid floating point artifacts
	rounded := math.Round(value*1e10) / 1e10
	s := strconv.FormatFloat(rounded, 'f', -1, 64)
	dotIdx := strings.Index(s, ".")
	if dotIdx < 0 {
		return 0
	}
	return len(s) - dotIdx - 1
}

// extractCellValue extracts a cell value from cell data.
func extractCellValue(cellType int, flags uint32, data []byte, stringMap map[uint32]string,
	richTextMap map[uint32]string, formatMap map[uint32]*cellFormat,
) string {
	// Get the cell format if available
	cf := getCellFormat(flags, data, formatMap)

	switch cellType {
	case cellTypeEmpty:
		return ""
	case cellTypeText:
		// String ID is at offset 0 if STRING_ID flag is set.
		if flags&cellFlagStringID != 0 && len(data) >= 4 {
			stringID := binary.LittleEndian.Uint32(data[0:4])
			if str, ok := stringMap[stringID]; ok {
				return str
			}
			return fmt.Sprintf("[string:%d]", stringID)
		}
		return ""
	case cellTypeNumber, cellTypeCurrency:
		// Double is at offset 0 if DOUBLE flag is set.
		if flags&cellFlagDouble != 0 && len(data) >= 8 {
			bits := binary.LittleEndian.Uint64(data[0:8])
			value := float64frombits(bits)
			return formatFloat(value, cf)
		}
		// Decimal128 format (16 bytes) - IEEE 754-2008 decimal floating point
		if flags&cellFlagDecimal128 != 0 && len(data) >= 16 {
			value := decodeDecimal128(data[0:16])
			return formatFloat(value, cf)
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
		// Rich text ID is at the offset determined by flags.
		if flags&cellFlagRichID != 0 && len(data) >= 4 {
			// Calculate offset to rich text ID by summing sizes of preceding flags.
			offset := 0
			for _, fs := range flagSizes {
				if flags&fs.flag != 0 {
					if fs.flag == cellFlagRichID {
						// Found the rich text ID
						if offset+4 <= len(data) {
							richID := binary.LittleEndian.Uint32(data[offset : offset+4])
							if str, ok := richTextMap[richID]; ok {
								return str
							}
							return fmt.Sprintf("[rich:%d]", richID)
						}
						break
					}
					offset += fs.size
				}
			}
		}
		return "[rich text]"
	case cellTypeDate:
		// Date is stored as seconds since Apple epoch (2001-01-01)
		// Check for DOUBLE flag (8 bytes) or DECIMAL128 flag (16 bytes)
		if flags&cellFlagDouble != 0 && len(data) >= 8 {
			bits := binary.LittleEndian.Uint64(data[0:8])
			seconds := float64frombits(bits)
			// Convert Apple epoch to Unix epoch (difference is 978307200 seconds)
			unixSeconds := int64(seconds) + 978307200
			t := time.Unix(unixSeconds, 0).UTC()
			return t.Format("2006-01-02")
		}
		return "[date]"
	case cellTypeDuration:
		// Duration is stored as seconds
		if flags&cellFlagDouble != 0 && len(data) >= 8 {
			bits := binary.LittleEndian.Uint64(data[0:8])
			seconds := float64frombits(bits)
			// Format as hours:minutes:seconds
			hours := int(seconds) / 3600
			minutes := (int(seconds) % 3600) / 60
			secs := int(seconds) % 60
			return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
		}
		return "[duration]"
	case cellTypeError:
		return "[error]"
	case cellTypeFormula:
		// Formula cells have cached values, but extracting them requires parsing the formula
		// For now, return empty string to avoid cluttering output
		return ""
	case cellTypeSpan:
		// Span cells are part of merged ranges
		return ""
	default:
		return fmt.Sprintf("[type:%d]", cellType)
	}
}

// float64frombits converts uint64 bits to float64.
func float64frombits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// decodeDecimal128 decodes a 128-bit decimal floating point number.
// Apple uses IEEE 754-2008 decimal128 format with binary integer significand.
// Layout (little-endian byte order):
//
//	b[15] bit 7       = sign
//	b[15] bits 0-6    = high 7 bits of biased exponent
//	b[14] bits 1-7    = low 7 bits of biased exponent
//	b[14] bit 0       = high bit of significand (bit 112)
//	b[13..0]          = low 112 bits of significand
//
// Biased exponent uses bias 6176 (0x1820).
func decodeDecimal128(data []byte) float64 {
	if len(data) < 16 {
		return 0
	}

	// Sign
	sign := 1.0
	if data[15]&0x80 != 0 {
		sign = -1.0
	}

	// Biased exponent (14 bits)
	biasedExp := uint32(data[15]&0x7f)<<7 | uint32(data[14]>>1)
	exp := int(biasedExp) - 6176

	// Significand (113 bits) accumulated as float64.
	// b[14] bit 0 is the highest bit; b[13..0] are the remaining bits.
	mantissa := float64(data[14] & 1)
	for i := 13; i >= 0; i-- {
		mantissa = mantissa*256 + float64(data[i])
	}

	if mantissa == 0 {
		return 0
	}

	return sign * mantissa * math.Pow10(exp)
}

// tableToMarkdown converts a table to markdown format.
func tableToMarkdown(table *Table) string {
	return renderMarkdownTable(table.Rows, table.Cols, table.Cells)
}

// renderMarkdownTable renders a markdown table from rows, cols, and cells.
func renderMarkdownTable(rows, cols int, cells [][]string) string {
	if rows == 0 || cols == 0 {
		return ""
	}

	var sb strings.Builder

	// Write header row
	sb.WriteString("| ")
	for col := range cols {
		sb.WriteString(cells[0][col])
		sb.WriteString(" | ")
	}
	sb.WriteString("\n")

	// Write separator
	sb.WriteString("|")
	for range cols {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Write data rows
	for row := 1; row < rows; row++ {
		sb.WriteString("| ")
		for col := range cols {
			sb.WriteString(cells[row][col])
			sb.WriteString(" | ")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
