package iwaread

import (
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/wmentor/iwaread/internal/base"
	"github.com/wmentor/iwaread/internal/tsp"
)

// Type IDs for Keynote-specific objects.
const (
	typeKNDocument    = 1    // KN.DocumentArchive
	typeKNShow        = 2    // KN.ShowArchive
	typeKNSlideNode   = 4    // KN.SlideNodeArchive
	typeKNSlide       = 5    // KN.SlideArchive
	typeKNSlideLegacy = 6    // KN.SlideArchive (legacy master-slide slot)
	typeKNPlaceholder = 7    // KN.PlaceholderArchive
	typeKNNote        = 15   // KN.NoteArchive
	typeKNHyperlink   = 2032 // Hyperlink object
)

// KN.DocumentArchive field numbers.
const (
	knDocShow = 2
)

// KN.ShowArchive field numbers.
const (
	knShowSlideTree = 3
)

// KN.SlideTreeArchive field numbers.
const (
	knSlideTreeSlides = 2
)

// KN.SlideNodeArchive field numbers.
const (
	knSlideNodeChildren = 1
	knSlideNodeSlide    = 2
)

// KN.SlideArchive field numbers.
const (
	knSlideTitlePlaceholder = 5
	knSlideBodyPlaceholder  = 6
	knSlideNote             = 27
)

// KN.PlaceholderArchive field numbers.
const (
	knPlaceholderSuper = 1
)

// TSWP.ShapeInfoArchive field numbers.
const (
	knShapeOwnedStorage      = 4
	knShapeDeprecatedStorage = 2 // For older files
)

// KN.NoteArchive field numbers.
const (
	knNoteContainedStorage = 1
)

// TSWP.StorageArchive field numbers.
const (
	knStorageText = 3
)

// ReadKeynote reads a .key file and returns its content as markdown.
// Each slide is rendered with its title as a heading, followed by body text,
// and speaker notes (if any) in a separate section.
func ReadKeynote(filename string) (string, error) {
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

	return extractKeynoteMarkdown(store)
}

func extractKeynoteMarkdown(store *tsp.ObjectStore) (string, error) {
	// Build hyperlink map (UUID -> URL)
	hyperlinkMap := buildHyperlinkMap(store)

	// Find KN.DocumentArchive
	docObj := store.FindByType(typeKNDocument)
	if docObj == nil {
		return "", fmt.Errorf("KN.DocumentArchive not found — not a Keynote document?")
	}

	// Get show reference
	showRef, err := docObj.Message.GetMessage(knDocShow)
	if err != nil || showRef == nil {
		return "", fmt.Errorf("show reference not found")
	}

	showID, ok := showRef.GetVarint(1)
	if !ok {
		return "", fmt.Errorf("could not get show ID")
	}

	showObj := store.Object(showID)
	if showObj == nil || showObj.Type != typeKNShow {
		return "", fmt.Errorf("show object not found")
	}

	// Get slide tree (inline, not a reference)
	slideTree, err := showObj.Message.GetMessage(knShowSlideTree)
	if err != nil || slideTree == nil {
		return "", fmt.Errorf("slide tree not found")
	}

	// Get slides list
	slideNodeRefs, err := slideTree.GetMessages(knSlideTreeSlides)
	if err != nil {
		return "", fmt.Errorf("failed to get slides: %w", err)
	}

	var sb strings.Builder
	slideNum := 0

	// Process each slide node
	for _, slideNodeRef := range slideNodeRefs {
		slideNodes := collectSlideNodes(store, slideNodeRef)
		for _, slideNode := range slideNodes {
			slideNum++
			slideMarkdown := extractSlideMarkdown(store, slideNode, slideNum, hyperlinkMap)
			sb.WriteString(slideMarkdown)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// buildHyperlinkMap builds a map of UUID -> URL from hyperlink objects.
func buildHyperlinkMap(store *tsp.ObjectStore) map[string]string {
	hyperlinkMap := make(map[string]string)

	for _, obj := range store.AllObjects() {
		if obj.Type != typeKNHyperlink {
			continue
		}

		// Field 1 contains UUID, field 2 contains URL
		uuid, ok := obj.Message.GetString(1)
		if !ok || uuid == "" {
			continue
		}

		url, ok := obj.Message.GetString(2)
		if !ok || url == "" {
			continue
		}

		hyperlinkMap[uuid] = url
	}

	return hyperlinkMap
}

// collectSlideNodes performs a depth-first traversal of slide nodes.
func collectSlideNodes(store *tsp.ObjectStore, nodeRef *base.RawMessage) []*tsp.IwaObject {
	var result []*tsp.IwaObject

	// Get slide node ID
	nodeID, ok := nodeRef.GetVarint(1)
	if !ok {
		return result
	}

	nodeObj := store.Object(nodeID)
	if nodeObj == nil || nodeObj.Type != typeKNSlideNode {
		return result
	}

	// Add this node first (pre-order traversal)
	result = append(result, nodeObj)

	// Recursively process children
	children, err := nodeObj.Message.GetMessages(knSlideNodeChildren)
	if err != nil {
		return result
	}

	for _, childRef := range children {
		childNodes := collectSlideNodes(store, childRef)
		result = append(result, childNodes...)
	}

	return result
}

// extractSlideMarkdown extracts markdown content from a single slide.
func extractSlideMarkdown(store *tsp.ObjectStore, slideNode *tsp.IwaObject, slideNum int, hyperlinkMap map[string]string) string {
	var sb strings.Builder

	// Get slide reference
	slideRef, err := slideNode.Message.GetMessage(knSlideNodeSlide)
	if err != nil || slideRef == nil {
		return ""
	}

	slideID, ok := slideRef.GetVarint(1)
	if !ok {
		return ""
	}

	slideObj := store.Object(slideID)
	if slideObj == nil || (slideObj.Type != typeKNSlide && slideObj.Type != typeKNSlideLegacy) {
		return ""
	}

	// Extract title
	title := extractPlaceholderText(store, slideObj, knSlideTitlePlaceholder, hyperlinkMap)

	// Extract body
	body := extractPlaceholderText(store, slideObj, knSlideBodyPlaceholder, hyperlinkMap)

	// Extract text from drawable shapes (type 2011)
	drawableTexts := extractDrawableTexts(store, slideObj, hyperlinkMap)

	// Extract tables
	tables := extractTablesFromSlide(store, slideObj, hyperlinkMap)

	// Extract notes
	notes := extractNoteText(store, slideObj, hyperlinkMap)

	// Write slide heading
	sb.WriteString("## Slide ")
	sb.WriteString(fmt.Sprintf("%d", slideNum))
	if title != "" {
		sb.WriteString(": ")
		sb.WriteString(title)
	}
	sb.WriteString("\n\n")

	// Write body
	if body != "" {
		sb.WriteString(body)
		sb.WriteString("\n\n")
	}

	// Write drawable texts
	for _, text := range drawableTexts {
		if text != "" {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	}

	// Write tables
	for i, table := range tables {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(tableToMarkdown(&table))
		sb.WriteString("\n\n")
	}

	// Write notes
	if notes != "" {
		sb.WriteString("**Notes:**\n")
		sb.WriteString(notes)
		sb.WriteString("\n")
	}

	return sb.String()
}

// extractTablesFromSlide extracts all tables from a slide.
func extractTablesFromSlide(store *tsp.ObjectStore, slideObj *tsp.IwaObject, hyperlinkMap map[string]string) []Table {
	var tables []Table

	// Get owned_drawables (field 7)
	drawables, err := slideObj.Message.GetMessages(7)
	if err != nil {
		return tables
	}

	for _, drawable := range drawables {
		refID, ok := drawable.GetVarint(1)
		if !ok {
			continue
		}

		drawableObj := store.Object(refID)
		if drawableObj == nil {
			continue
		}

		// Check if it's a table (type 6000)
		if drawableObj.Type != typeTableInfo {
			continue
		}

		// Extract table using Keynote-specific function
		table, err := extractKeynoteTable(store, drawableObj, hyperlinkMap)
		if err != nil {
			continue
		}

		tables = append(tables, *table)
	}

	return tables
}

// extractDrawableTexts extracts text from drawable shape objects (type 2011).
func extractDrawableTexts(store *tsp.ObjectStore, slideObj *tsp.IwaObject, hyperlinkMap map[string]string) []string {
	var texts []string

	// Get owned_drawables (field 7)
	drawables, err := slideObj.Message.GetMessages(7)
	if err != nil {
		return texts
	}

	for _, drawable := range drawables {
		refID, ok := drawable.GetVarint(1)
		if !ok {
			continue
		}

		drawableObj := store.Object(refID)
		if drawableObj == nil {
			continue
		}

		// Check if it's a shape (type 2011)
		if drawableObj.Type != 2011 {
			continue
		}

		// Try to find storage reference in field 2 or 4
		for _, fieldNo := range []int{2, 4} {
			storageRef, err := drawableObj.Message.GetMessage(fieldNo)
			if err != nil || storageRef == nil {
				continue
			}

			storageID, ok := storageRef.GetVarint(1)
			if !ok {
				continue
			}

			storageObj := store.Object(storageID)
			if storageObj == nil || storageObj.Type != 2001 {
				continue
			}

			// Get text from storage
			storageTexts := storageObj.Message.GetStrings(3)
			if len(storageTexts) > 0 && storageTexts[0] != "" {
				text := storageTexts[0]

				// Apply hyperlinks first (before character formatting to preserve positions)
				text = applyHyperlinks(store, storageObj, text, hyperlinkMap)

				// Apply character formatting (bold, italic) after hyperlinks
				charStyleTable, _ := storageObj.Message.GetMessage(8)
				if charStyleTable != nil {
					text = applyDrawableCharacterFormatting(store, text, charStyleTable)
				}

				// Check if this storage has list styles
				listStyleTable, _ := storageObj.Message.GetMessage(7)
				if listStyleTable != nil {
					// Process text with list formatting
					text = processDrawableTextWithListStyles(store, text, listStyleTable)
				}

				texts = append(texts, text)
				break // Found text, no need to check other fields
			}
		}
	}

	return texts
}

// processDrawableTextWithListStyles processes text from drawable objects and applies list formatting.
func processDrawableTextWithListStyles(store *tsp.ObjectStore, text string, listStyleTable *base.RawMessage) string {
	// Split text into paragraphs
	paragraphs := splitKeynoteParagraphs(text)
	if len(paragraphs) == 0 {
		return text
	}

	var result strings.Builder
	charPos := 0

	for i, para := range paragraphs {
		if para == "" {
			if i < len(paragraphs)-1 {
				result.WriteString("\n")
			}
			continue
		}

		// Check if this paragraph is a list item
		isList := isKeynoteListParagraph(nil, listStyleTable, store, i, text)

		if isList {
			result.WriteString("- ")
		}

		result.WriteString(para)

		if i < len(paragraphs)-1 {
			result.WriteString("\n")
			if !isList {
				result.WriteString("\n")
			}
		}

		// Update charPos for next paragraph
		charPos += len([]rune(para)) + 1 // +1 for newline
	}

	return result.String()
}

// applyDrawableCharacterFormatting applies bold and italic formatting to text based on character style table.
func applyDrawableCharacterFormatting(store *tsp.ObjectStore, text string, charStyleTable *base.RawMessage) string {
	if charStyleTable == nil || text == "" {
		return text
	}

	// Get all character style entries
	entries, err := charStyleTable.GetMessages(1)
	if err != nil || len(entries) == 0 {
		return text
	}

	// Build a list of style changes
	type styleChange struct {
		position int
		bold     bool
		italic   bool
	}

	var changes []styleChange

	for _, entry := range entries {
		charIdx, _ := entry.GetUint(1)
		objRef, _ := entry.GetMessage(2)

		if objRef == nil {
			// No style reference means reset to normal
			changes = append(changes, styleChange{position: charIdx, bold: false, italic: false})
			continue
		}

		objID, ok := objRef.GetVarint(1)
		if !ok {
			continue
		}

		styleObj := store.Object(objID)
		if styleObj == nil || styleObj.Type != 2021 {
			continue
		}

		// Get character properties (field 11)
		charProps, err := styleObj.Message.GetMessage(11)
		if err != nil || charProps == nil {
			changes = append(changes, styleChange{position: charIdx, bold: false, italic: false})
			continue
		}

		bold, _ := charProps.GetBool(1)
		italic, _ := charProps.GetBool(2)

		changes = append(changes, styleChange{position: charIdx, bold: bold, italic: italic})
	}

	if len(changes) == 0 {
		return text
	}

	// Sort changes by position
	for i := 0; i < len(changes); i++ {
		for j := i + 1; j < len(changes); j++ {
			if changes[i].position > changes[j].position {
				changes[i], changes[j] = changes[j], changes[i]
			}
		}
	}

	// Convert text to runes for proper character indexing
	runes := []rune(text)
	var result strings.Builder

	currentStyleIdx := 0
	currentBold := false
	currentItalic := false
	i := 0

	for i < len(runes) {
		// Check if we need to change style at this position
		if currentStyleIdx < len(changes) && i == changes[currentStyleIdx].position {
			newBold := changes[currentStyleIdx].bold
			newItalic := changes[currentStyleIdx].italic

			// Only update formatting if style actually changes
			if newBold != currentBold || newItalic != currentItalic {
				// Close previous formatting if active
				if currentBold || currentItalic {
					if currentBold && currentItalic {
						result.WriteString("***")
					} else if currentBold {
						result.WriteString("**")
					} else if currentItalic {
						result.WriteString("*")
					}
				}

				// Update current style
				currentBold = newBold
				currentItalic = newItalic

				// Open new formatting if needed
				if currentBold || currentItalic {
					if currentBold && currentItalic {
						result.WriteString("***")
					} else if currentBold {
						result.WriteString("**")
					} else if currentItalic {
						result.WriteString("*")
					}
				}
			}

			currentStyleIdx++
		}

		// Write current character
		result.WriteRune(runes[i])
		i++
	}

	// Close any remaining formatting
	if currentBold || currentItalic {
		if currentBold && currentItalic {
			result.WriteString("***")
		} else if currentBold {
			result.WriteString("**")
		} else if currentItalic {
			result.WriteString("*")
		}
	}

	return result.String()
}

// extractKeynoteTable extracts a table from a Keynote document.
// Keynote uses a tile-based binary storage format similar to Numbers.
func extractKeynoteTable(store *tsp.ObjectStore, tableInfo *tsp.IwaObject, hyperlinkMap map[string]string) (*Table, error) {
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

	// Get data store (field 4) - this is the "grid" in libetonyek
	dataStoreMsg, err := model.Message.GetMessage(4)
	if err != nil || dataStoreMsg == nil {
		return nil, fmt.Errorf("no data store message")
	}

	// Build data lists from the grid
	// Field 4: simpleTextList (string table)
	stringMap, err := buildKeynoteDataList(store, dataStoreMsg, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to build string map: %w", err)
	}

	// Field 17: formattedTextList (rich text references)
	richTextMap, err := buildKeynoteRichTextMap(store, dataStoreMsg, 17, hyperlinkMap)
	if err != nil {
		richTextMap = make(map[uint32]string)
	}

	// Field 11: formatList (old format)
	formatMap := buildKeynoteFormatMap(store, dataStoreMsg, 11)

	// Field 22: newFormatList (new format)
	newFormatMap := buildKeynoteFormatMap(store, dataStoreMsg, 22)

	// Get tiles from field 3 (TileStorage)
	tilesMsg, err := dataStoreMsg.GetMessage(3)
	if err != nil || tilesMsg == nil {
		return nil, fmt.Errorf("no tiles message")
	}

	// Get tile references (field 1 in TileStorage)
	tileRefs, err := tilesMsg.GetMessages(1)
	if err != nil {
		return nil, fmt.Errorf("error getting tile refs: %w", err)
	}

	// Get tile row decals from field 9
	tileDecals := make(map[uint32]uint32)
	if decalsMsg, err := dataStoreMsg.GetMessage(9); err == nil && decalsMsg != nil {
		if decalEntries, err := decalsMsg.GetMessages(1); err == nil {
			for _, entry := range decalEntries {
				decal, _ := entry.GetUint(1)
				id, _ := entry.GetUint(2)
				if id != 0 {
					tileDecals[uint32(id)] = uint32(decal)
				}
			}
		}
	}

	// Initialize cells array
	cells := make([][]string, rows)
	for i := range cells {
		cells[i] = make([]string, cols)
	}

	// Process each tile
	for _, tileRef := range tileRefs {
		// Get tile ID (field 1)
		tileIDRaw, _ := tileRef.GetUint(1)
		tileID := uint32(tileIDRaw)

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

		// Get row decal for this tile
		decalY := uint32(0)
		if d, ok := tileDecals[tileID]; ok {
			decalY = d
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
			actualRow := uint64(rowIndex) + uint64(decalY)

			if actualRow >= uint64(rows) {
				continue
			}

			// Try new format first (field 6 = data, field 7 = offsets)
			dataBuffer, hasData := rowInfo.GetBytes(6)
			offsetBuffer, hasOffsets := rowInfo.GetBytes(7)
			useNewFormat := hasData && hasOffsets

			// Fall back to old format (field 3 = data, field 4 = offsets)
			if !useNewFormat {
				dataBuffer, hasData = rowInfo.GetBytes(3)
				offsetBuffer, hasOffsets = rowInfo.GetBytes(4)
			}

			if !hasData || !hasOffsets {
				continue
			}

			// Check if field 8 (factor flag) exists
			useFactor4 := false
			if field8Msg, err := rowInfo.GetMessage(8); err == nil && field8Msg != nil {
				if val, ok := field8Msg.GetBool(1); ok && val {
					useFactor4 = true
				}
			}

			// Parse column offsets
			offsets := parseKeynoteColumnOffsets(offsetBuffer, len(dataBuffer), useNewFormat, useFactor4)

			// Process each column
			for col, begPos := range offsets {
				if col >= int(cols) {
					continue
				}

				// Find end position (next column's start or end of buffer)
				endPos := len(dataBuffer)
				for nextCol := col + 1; nextCol < int(cols); nextCol++ {
					if nextPos, ok := offsets[nextCol]; ok {
						endPos = nextPos
						break
					}
				}

				if begPos >= len(dataBuffer) || begPos >= endPos {
					continue
				}

				// Parse cell definition
				cellValue := parseKeynoteCellDefinition(
					dataBuffer[begPos:endPos],
					useNewFormat,
					stringMap,
					richTextMap,
					formatMap,
					newFormatMap,
				)

				cells[int(actualRow)][col] = cellValue
			}
		}
	}

	// Fallback: if all cells are empty, try to extract from Field 32 in object type 6003
	allEmpty := true
	for _, row := range cells {
		for _, cell := range row {
			if cell != "" {
				allEmpty = false
				break
			}
		}
		if !allEmpty {
			break
		}
	}

	if allEmpty {
		// Build simple string map for fallback
		simpleStringMap := make(map[uint32]string)
		for k, v := range stringMap {
			if s, ok := v.(string); ok {
				simpleStringMap[k] = s
			}
		}
		extractFromField32(store, model, simpleStringMap, cells, int(rows), int(cols))
	}

	return &Table{
		Rows:  rows,
		Cols:  cols,
		Cells: cells,
	}, nil
}

// parseKeynoteColumnOffsets parses the column offset buffer.
// Each offset is a 16-bit value indicating where each column's data starts.
func parseKeynoteColumnOffsets(offsetBuffer []byte, dataLength int, useNewFormat bool, useFactor4 bool) map[int]int {
	offsets := make(map[int]int)
	col := 0
	factor := 1
	if useNewFormat && useFactor4 {
		factor = 4
	}

	for i := 0; i+1 < len(offsetBuffer); i += 2 {
		offset := binary.LittleEndian.Uint16(offsetBuffer[i : i+2])
		actualOffset := int(offset) * factor

		if actualOffset < dataLength && actualOffset+4 < dataLength {
			offsets[col] = actualOffset
		} else if offset != 0xffff {
			// Invalid offset, skip
		}
		col++
	}

	return offsets
}

// parseKeynoteCellDefinition parses a cell definition from the binary data.
func parseKeynoteCellDefinition(
	data []byte,
	useNewFormat bool,
	stringMap map[uint32]any,
	richTextMap map[uint32]string,
	formatMap map[uint32]*cellFormat,
	newFormatMap map[uint32]*cellFormat,
) string {
	if len(data) < 12 {
		return ""
	}

	// byte 0: unknown (4?)
	// byte 1: cell type
	cellType := int(data[1])

	if useNewFormat {
		return parseKeynoteCellNewFormat(data, cellType, stringMap, richTextMap, formatMap, newFormatMap)
	}
	return parseKeynoteCellOldFormat(data, cellType, stringMap, richTextMap, formatMap)
}

// parseKeynoteCellOldFormat parses a cell in old format (16-bit flags).
func parseKeynoteCellOldFormat(
	data []byte,
	cellType int,
	stringMap map[uint32]any,
	richTextMap map[uint32]string,
	formatMap map[uint32]*cellFormat,
) string {
	if len(data) < 12 {
		return ""
	}

	// bytes 4-5: flags (16-bit)
	flags := binary.LittleEndian.Uint16(data[4:6])

	// Skip to data area (bytes 6-11 are skipped)
	pos := 12

	var textID, textFormattedID, formatID uint32
	var numberValue float64
	var numberSet bool

	if flags&0x2 != 0 && pos+4 <= len(data) {
		pos += 4 // cell style
	}
	if flags&0x80 != 0 && pos+4 <= len(data) {
		pos += 4 // paragraph style
	}
	if flags&0x800 != 0 && pos+4 <= len(data) {
		pos += 4 // condition
	}
	if flags&0x400 != 0 && pos+4 <= len(data) {
		pos += 4 // condition 2
	}
	if flags&0x4 != 0 && pos+4 <= len(data) {
		formatID = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if flags&0x8 != 0 && pos+4 <= len(data) {
		pos += 4 // formula
	}
	if flags&0x1000 != 0 && pos+4 <= len(data) {
		pos += 4 // comment
	}
	if flags&0x10 != 0 && pos+4 <= len(data) {
		textID = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if flags&0x20 != 0 && pos+8 <= len(data) {
		bits := binary.LittleEndian.Uint64(data[pos : pos+8])
		numberValue = float64frombits(bits)
		numberSet = true
		pos += 8
	}
	if flags&0x40 != 0 && pos+8 <= len(data) {
		bits := binary.LittleEndian.Uint64(data[pos : pos+8])
		numberValue = float64frombits(bits)
		numberSet = true
		pos += 8
	}
	if flags&0x200 != 0 && pos+4 <= len(data) {
		textFormattedID = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}

	return extractKeynoteCellValueFromData(cellType, textID, textFormattedID, numberValue, numberSet, formatID, stringMap, richTextMap, formatMap)
}

// parseKeynoteCellNewFormat parses a cell in new format (32-bit flags).
func parseKeynoteCellNewFormat(
	data []byte,
	cellType int,
	stringMap map[uint32]any,
	richTextMap map[uint32]string,
	formatMap map[uint32]*cellFormat,
	newFormatMap map[uint32]*cellFormat,
) string {
	if len(data) < 12 {
		return ""
	}

	// bytes 8-11: flags (32-bit)
	flags := binary.LittleEndian.Uint32(data[8:12])

	pos := 12

	var textID, textFormattedID, formatID uint32
	var numberValue float64
	var numberSet bool
	var resType uint32

	if flags&0x1 != 0 && pos+16 <= len(data) {
		// Decimal128 number (16 bytes)
		// Decode as: mantissa * 10^((exponent-12352)/2)
		// Read 7 uint16 values for mantissa, 1 uint16 for exponent
		mantissa := 0.0
		decal := 1.0
		for range 7 {
			if pos+2 <= len(data) {
				val := binary.LittleEndian.Uint16(data[pos : pos+2])
				mantissa += decal * float64(val)
				decal *= 65536
				pos += 2
			}
		}
		if pos+2 <= len(data) {
			exponent := binary.LittleEndian.Uint16(data[pos : pos+2])
			pos += 2
			if exponent&1 != 0 {
				mantissa += decal
				exponent &= 0xfffe
			}
			if exponent&0x8000 != 0 {
				mantissa *= -1
				exponent &= 0x7fff
			}
			numberValue = mantissa * float64(pow10(int(exponent-12352)/2))
			numberSet = true
		}
	}
	if flags&0x2 != 0 && pos+8 <= len(data) {
		// Bool (8 bytes double)
		bits := binary.LittleEndian.Uint64(data[pos : pos+8])
		numberValue = float64frombits(bits)
		numberSet = true
		pos += 8
	}
	if flags&0x4 != 0 && pos+8 <= len(data) {
		// Date (8 bytes double)
		bits := binary.LittleEndian.Uint64(data[pos : pos+8])
		numberValue = float64frombits(bits)
		numberSet = true
		pos += 8
	}
	if flags&0x8 != 0 && pos+4 <= len(data) {
		textID = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if flags&0x10 != 0 && pos+4 <= len(data) {
		textFormattedID = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	if flags&0x20 != 0 && pos+4 <= len(data) {
		pos += 4 // cell style
	}
	if flags&0x40 != 0 && pos+4 <= len(data) {
		pos += 4 // paragraph style
	}
	if flags&0x80 != 0 && pos+4 <= len(data) {
		pos += 4 // condition
	}
	if flags&0x100 != 0 && pos+4 <= len(data) {
		pos += 4 // condition unknown
	}
	if flags&0x200 != 0 && pos+4 <= len(data) {
		pos += 4 // formula
	}
	if flags&0x400 != 0 && pos+4 <= len(data) {
		pos += 4 // button menu
	}
	if flags&0x800 != 0 && pos+4 <= len(data) {
		pos += 4 // unknown
	}
	if flags&0x1000 != 0 && pos+4 <= len(data) {
		resType = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}

	// Format IDs (6 possible, flags 0x2000-0x20000)
	for w := range 6 {
		flagBit := uint32(0x2000) << w
		if flags&flagBit != 0 && pos+4 <= len(data) {
			id := binary.LittleEndian.Uint32(data[pos : pos+4])
			pos += 4
			// Use this format ID if it matches the result type
			if w+1 == int(resType) {
				formatID = id
			}
		}
	}

	if flags&0x80000 != 0 && pos+4 <= len(data) {
		pos += 4 // comment
	}

	// Use new format map if available
	if len(newFormatMap) > 0 {
		return extractKeynoteCellValueFromData(cellType, textID, textFormattedID, numberValue, numberSet, formatID, stringMap, richTextMap, newFormatMap)
	}
	return extractKeynoteCellValueFromData(cellType, textID, textFormattedID, numberValue, numberSet, formatID, stringMap, richTextMap, formatMap)
}

// extractKeynoteCellValueFromData extracts the cell value based on the parsed data.
func extractKeynoteCellValueFromData(
	cellType int,
	textID, textFormattedID uint32,
	numberValue float64,
	numberSet bool,
	formatID uint32,
	stringMap map[uint32]any,
	richTextMap map[uint32]string,
	formatMap map[uint32]*cellFormat,
) string {
	// Get cell format
	var cf *cellFormat
	if formatID != 0 {
		cf = formatMap[formatID]
	}

	// Check if we have text
	if textID != 0 {
		if val, ok := stringMap[textID]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}

	if textFormattedID != 0 {
		if s, ok := richTextMap[textFormattedID]; ok {
			return s
		}
	}

	// Handle based on cell type
	switch cellType {
	case 0: // empty
		return ""
	case 2, 8, 10: // number, nan, currency
		if numberSet {
			return formatFloat(numberValue, cf)
		}
		return ""
	case 3, 9: // text, text zone
		return ""
	case 5: // date
		if numberSet {
			unixSeconds := int64(numberValue) + 978307200
			t := time.Unix(unixSeconds, 0).UTC()
			return t.Format("2006-01-02")
		}
		return ""
	case 6: // bool
		if numberSet {
			if numberValue != 0 {
				return "TRUE"
			}
			return "FALSE"
		}
		return ""
	case 7: // duration
		if numberSet {
			hours := int(numberValue) / 3600
			minutes := (int(numberValue) % 3600) / 60
			secs := int(numberValue) % 60
			return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
		}
		return ""
	default:
		return ""
	}
}

// pow10 returns 10^n for integer n.
func pow10(n int) float64 {
	if n == 0 {
		return 1
	}
	result := 1.0
	if n > 0 {
		for range n {
			result *= 10
		}
	} else {
		for i := 0; i > n; i-- {
			result /= 10
		}
	}
	return result
}

// buildKeynoteDataList builds a map from a data list in the grid.
func buildKeynoteDataList(store *tsp.ObjectStore, dataStoreMsg *base.RawMessage, fieldNo int) (map[uint32]any, error) {
	ref, err := dataStoreMsg.GetMessage(fieldNo)
	if err != nil || ref == nil {
		return make(map[uint32]any), nil
	}

	id, ok := ref.GetVarint(1)
	if !ok {
		return make(map[uint32]any), nil
	}

	obj := store.Object(id)
	if obj == nil {
		return make(map[uint32]any), nil
	}

	result := make(map[uint32]any)
	entries, err := obj.Message.GetMessages(3)
	if err != nil {
		return result, nil
	}

	for _, entry := range entries {
		key, _ := entry.GetUint(1)
		// Try to get string value (field 3)
		if valueBytes, ok := entry.GetBytes(3); ok {
			result[uint32(key)] = string(valueBytes)
		} else if subMsg, err := entry.GetMessage(4); err == nil && subMsg != nil {
			// Try to get reference (field 4 -> field 1)
			if objID, ok := subMsg.GetUint(1); ok {
				result[uint32(key)] = uint32(objID)
			}
		}
	}

	return result, nil
}

// buildKeynoteRichTextMap builds a map of rich text references.
func buildKeynoteRichTextMap(store *tsp.ObjectStore, dataStoreMsg *base.RawMessage, fieldNo int,
	hyperlinkMap map[string]string,
) (map[uint32]string, error) {
	ref, err := dataStoreMsg.GetMessage(fieldNo)
	if err != nil || ref == nil {
		return make(map[uint32]string), nil
	}

	id, ok := ref.GetVarint(1)
	if !ok {
		return make(map[uint32]string), nil
	}

	obj := store.Object(id)
	if obj == nil {
		return make(map[uint32]string), nil
	}

	result := make(map[uint32]string)
	entries, err := obj.Message.GetMessages(3)
	if err != nil {
		return result, nil
	}

	for _, entry := range entries {
		key, _ := entry.GetUint(1)

		// Try field 9 first (Keynote rich text chain: entry -> field 9 -> obj 6218 -> field 1 -> obj 2001 -> field 3)
		if subMsg, err := entry.GetMessage(9); err == nil && subMsg != nil {
			if obj6218ID, ok := subMsg.GetVarint(1); ok {
				obj6218 := store.Object(uint64(obj6218ID))
				if obj6218 != nil {
					// Object 6218 field 1 -> text storage (type 2001)
					if field1Ref, err := obj6218.Message.GetMessage(1); err == nil && field1Ref != nil {
						if textStorageID, ok := field1Ref.GetVarint(1); ok {
							textStorage := store.Object(textStorageID)
							if textStorage != nil {
								if texts := textStorage.Message.GetStrings(3); len(texts) > 0 {
									text := texts[0]
									// Apply hyperlinks first (before character formatting to preserve positions)
									text = applyHyperlinks(store, textStorage, text, hyperlinkMap)
									// Apply character formatting (bold, italic) after hyperlinks
									charStyleTable, _ := textStorage.Message.GetMessage(8)
									if charStyleTable != nil {
										text = applyDrawableCharacterFormatting(store, text, charStyleTable)
									}
									result[uint32(key)] = text
									continue
								}
							}
						}
					}
				}
			}
		}

		// Fall back to field 4 (Pages-style)
		if subMsg, err := entry.GetMessage(4); err == nil && subMsg != nil {
			if objID, ok := subMsg.GetUint(1); ok {
				textObj := store.Object(uint64(objID))
				if textObj != nil {
					if texts := textObj.Message.GetStrings(3); len(texts) > 0 {
						text := texts[0]
						// Apply hyperlinks first (before character formatting to preserve positions)
						text = applyHyperlinks(store, textObj, text, hyperlinkMap)
						// Apply character formatting (bold, italic) after hyperlinks
						charStyleTable, _ := textObj.Message.GetMessage(8)
						if charStyleTable != nil {
							text = applyDrawableCharacterFormatting(store, text, charStyleTable)
						}
						result[uint32(key)] = text
					}
				}
			}
		}
	}

	return result, nil
}

// buildKeynoteFormatMap builds a map of cell formats.
func buildKeynoteFormatMap(store *tsp.ObjectStore, dataStoreMsg *base.RawMessage, fieldNo int) map[uint32]*cellFormat {
	ref, err := dataStoreMsg.GetMessage(fieldNo)
	if err != nil || ref == nil {
		return make(map[uint32]*cellFormat)
	}

	id, ok := ref.GetVarint(1)
	if !ok {
		return make(map[uint32]*cellFormat)
	}

	obj := store.Object(id)
	if obj == nil {
		return make(map[uint32]*cellFormat)
	}

	result := make(map[uint32]*cellFormat)
	entries, err := obj.Message.GetMessages(3)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		key, _ := entry.GetUint(1)
		if subMsg, err := entry.GetMessage(4); err == nil && subMsg != nil {
			if objID, ok := subMsg.GetUint(1); ok {
				// Create a placeholder format
				result[uint32(key)] = &cellFormat{}
				_ = objID // Would need to parse format object for full implementation
			}
		}
	}

	return result
}

// extractFromField32 attempts to extract cell data from Field 32 of object type 6003.
// This is a fallback for tables where cell_storage contains only empty cells.
func extractFromField32(store *tsp.ObjectStore, model *tsp.IwaObject, stringMap map[uint32]string,
	cells [][]string, rows, cols int,
) {
	// Get field 3 reference (object 6003)
	field3Ref, err := model.Message.GetMessage(3)
	if err != nil || field3Ref == nil {
		return
	}

	field3ID, ok := field3Ref.GetVarint(1)
	if !ok {
		return
	}

	field3Obj := store.Object(field3ID)
	if field3Obj == nil {
		return
	}

	// Get field 11 (large data container)
	field11Msg, err := field3Obj.Message.GetMessage(11)
	if err != nil || field11Msg == nil {
		return
	}

	// Get field 32 (cell data entries)
	field32Msg, err := field11Msg.GetMessage(32)
	if err != nil || field32Msg == nil {
		return
	}

	// Get all field 3 entries
	field3Entries, err := field32Msg.GetMessages(3)
	if err != nil || len(field3Entries) == 0 {
		return
	}

	// Extract Field 4 values and map to string table
	for i, entry := range field3Entries {
		field4Val, ok := entry.GetUint(4)
		if !ok {
			continue
		}

		// Try to find in string map
		if str, ok := stringMap[uint32(field4Val)]; ok {
			// Map entry index to cell position
			// Entry 0 -> cell (0, 0), Entry 1 -> cell (0, 1), etc.
			row := i / int(cols)
			col := i % int(cols)

			if row < int(rows) && col < int(cols) && cells[row][col] == "" {
				cells[row][col] = str
			}
		}
	}
}

// extractPlaceholderText extracts text from a placeholder (title or body).
func extractPlaceholderText(store *tsp.ObjectStore, slideObj *tsp.IwaObject, fieldNum int, hyperlinkMap map[string]string) string {
	// Get placeholder reference
	placeholderRef, err := slideObj.Message.GetMessage(fieldNum)
	if err != nil || placeholderRef == nil {
		return ""
	}

	placeholderID, ok := placeholderRef.GetVarint(1)
	if !ok {
		return ""
	}

	placeholderObj := store.Object(placeholderID)
	if placeholderObj == nil || placeholderObj.Type != typeKNPlaceholder {
		return ""
	}

	// Get super (ShapeInfoArchive) - this is an INLINE message, not a reference
	superMsg, err := placeholderObj.Message.GetMessage(knPlaceholderSuper)
	if err != nil || superMsg == nil {
		return ""
	}

	// Try owned_storage (field 4 in super)
	storageRef, err := superMsg.GetMessage(knShapeOwnedStorage)
	if err != nil || storageRef == nil {
		// Try deprecated_storage (field 2 in super) for older files
		storageRef, err = superMsg.GetMessage(knShapeDeprecatedStorage)
		if err != nil || storageRef == nil {
			return ""
		}
	}

	storageID, ok := storageRef.GetVarint(1)
	if !ok {
		return ""
	}

	storageObj := store.Object(storageID)
	if storageObj == nil {
		return ""
	}

	// Get text
	texts := storageObj.Message.GetStrings(knStorageText)
	if len(texts) == 0 {
		return ""
	}

	text := texts[0]
	if text == "" {
		return ""
	}

	// Apply hyperlinks first (before character formatting to preserve positions)
	text = applyHyperlinks(store, storageObj, text, hyperlinkMap)

	// Apply character formatting (bold, italic) after hyperlinks
	charStyleTable, _ := storageObj.Message.GetMessage(8)
	if charStyleTable != nil {
		text = applyDrawableCharacterFormatting(store, text, charStyleTable)
	}

	// Get paragraph style table (field 5) and list style table (field 7)
	paraStyleTable, _ := storageObj.Message.GetMessage(5)
	listStyleTable, _ := storageObj.Message.GetMessage(7)

	// Split text into paragraphs and apply list formatting
	paragraphs := splitKeynoteParagraphs(text)
	var result strings.Builder

	for i, para := range paragraphs {
		if para == "" {
			if i < len(paragraphs)-1 {
				result.WriteString("\n")
			}
			continue
		}

		// Check if this paragraph is a list item
		isList := isKeynoteListParagraph(paraStyleTable, listStyleTable, store, i, text)

		if isList {
			result.WriteString("- ")
		}

		result.WriteString(para)

		if i < len(paragraphs)-1 {
			result.WriteString("\n")
			if !isList {
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

// applyHyperlinks applies hyperlinks to text based on hyperlink information in the storage object.
func applyHyperlinks(store *tsp.ObjectStore, storageObj *tsp.IwaObject, text string, hyperlinkMap map[string]string) string {
	if len(hyperlinkMap) == 0 {
		return text
	}

	// Check field 11 first (for table cell text) - it has correct positions
	hyperlinkMsg, err := storageObj.Message.GetMessage(11)
	if err == nil && hyperlinkMsg != nil {
		text = applyHyperlinksFromField(store, storageObj, text, hyperlinkMap, 11)
		// If we found hyperlinks in field 11, return early
		if text != extractTextWithoutHyperlinks(storageObj) {
			return text
		}
	}

	// Fall back to field 20 (for placeholder text) - it may have incorrect positions
	hyperlinkMsg, err = storageObj.Message.GetMessage(20)
	if err == nil && hyperlinkMsg != nil {
		text = applyHyperlinksFromField(store, storageObj, text, hyperlinkMap, 20)
	}

	return text
}

// extractTextWithoutHyperlinks extracts the original text without any hyperlink modifications.
// This is a helper function to check if hyperlinks were applied.
func extractTextWithoutHyperlinks(storageObj *tsp.IwaObject) string {
	if texts := storageObj.Message.GetStrings(3); len(texts) > 0 {
		return texts[0]
	}
	return ""
}

// applyHyperlinksFromField applies hyperlinks from a specific field (20 or 11).
func applyHyperlinksFromField(store *tsp.ObjectStore, storageObj *tsp.IwaObject, text string, hyperlinkMap map[string]string, fieldNo int) string {
	hyperlinkMsg, err := storageObj.Message.GetMessage(fieldNo)
	if err != nil || hyperlinkMsg == nil {
		return text
	}

	// Parse hyperlink entries from the field
	hyperlinkEntries, err := hyperlinkMsg.GetMessages(1)
	if err != nil || len(hyperlinkEntries) == 0 {
		return text
	}

	// Build a list of hyperlink replacements
	type hyperlinkReplacement struct {
		start int
		end   int
		url   string
	}

	var replacements []hyperlinkReplacement

	// First pass: collect all positions and their associated URLs
	type positionInfo struct {
		position int
		url      string
	}
	var positions []positionInfo

	for _, entry := range hyperlinkEntries {
		// Field 1 contains position
		start, ok := entry.GetUint(1)
		if !ok {
			continue
		}

		// Field 2 contains the reference to hyperlink object (type 2032)
		refMsg, err := entry.GetMessage(2)
		if err != nil || refMsg == nil {
			// This is just a position marker without a hyperlink
			positions = append(positions, positionInfo{position: int(start), url: ""})
			continue
		}

		// Get the hyperlink object ID
		hyperlinkObjID, ok := refMsg.GetVarint(1)
		if !ok {
			positions = append(positions, positionInfo{position: int(start), url: ""})
			continue
		}

		// Get the hyperlink object
		hyperlinkObj := store.Object(uint64(hyperlinkObjID))
		if hyperlinkObj == nil || hyperlinkObj.Type != typeKNHyperlink {
			positions = append(positions, positionInfo{position: int(start), url: ""})
			continue
		}

		// Get URL from field 2
		url, ok := hyperlinkObj.Message.GetString(2)
		if !ok || url == "" {
			positions = append(positions, positionInfo{position: int(start), url: ""})
			continue
		}

		positions = append(positions, positionInfo{position: int(start), url: url})
	}

	// Second pass: create replacements for hyperlinks
	for i, posInfo := range positions {
		if posInfo.url == "" {
			continue
		}

		// Find the end position (next position in the list)
		end := len(text)
		if i+1 < len(positions) {
			end = positions[i+1].position
		}

		if end > posInfo.position && posInfo.position < len(text) {
			replacements = append(replacements, hyperlinkReplacement{
				start: posInfo.position,
				end:   end,
				url:   posInfo.url,
			})
		}
	}

	if len(replacements) == 0 {
		return text
	}

	// Apply replacements in reverse order to preserve positions
	for _, repl := range slices.Backward(replacements) {
		linkedText := text[repl.start:repl.end]
		replacement := fmt.Sprintf("[%s](%s)", linkedText, repl.url)
		text = text[:repl.start] + replacement + text[repl.end:]
	}

	return text
}

// splitKeynoteParagraphs splits text by paragraph separators.
func splitKeynoteParagraphs(text string) []string {
	var result []string
	start := 0
	for i := 0; i < len(text); i++ {
		ch := text[i]
		// Paragraph separators: newline, or special control characters
		if ch == '\n' || ch == 0x04 || ch == 0x05 || ch == 0x0c {
			result = append(result, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		result = append(result, text[start:])
	}
	return result
}

// isKeynoteListParagraph checks if a paragraph at the given index is a list item.
func isKeynoteListParagraph(paraStyleTable, listStyleTable *base.RawMessage, store *tsp.ObjectStore, paraIndex int, fullText string) bool {
	if listStyleTable == nil {
		return false
	}

	// Calculate character position for this paragraph
	charPos := 0
	for range paraIndex {
		// Find next paragraph separator
		for charPos < len(fullText) {
			ch := fullText[charPos]
			charPos++
			if ch == '\n' || ch == 0x04 || ch == 0x05 || ch == 0x0c {
				break
			}
		}
	}

	// Get list style at this position
	listStyleID := getEffectiveObject(listStyleTable, charPos)
	if listStyleID == 0 {
		return false
	}

	// Get list style identifier
	listStyleIdentifier := getKeynoteStyleIdentifier(store, listStyleID)

	// If identifier exists and doesn't contain "None", it's a list
	return listStyleIdentifier != "" && !strings.Contains(listStyleIdentifier, "None")
}

// getKeynoteStyleIdentifier returns the identifier for a Keynote style object.
// Keynote styles have a different structure than Pages styles.
func getKeynoteStyleIdentifier(store *tsp.ObjectStore, styleID uint64) string {
	if styleID == 0 {
		return ""
	}
	obj := store.Object(styleID)
	if obj == nil {
		return ""
	}

	// Get field 1 as bytes (this is a protobuf-encoded sub-message)
	b, ok := obj.Message.GetBytes(1)
	if !ok {
		return ""
	}

	// Parse as protobuf message (level 1)
	subMsg, err := base.ParseMessage(b)
	if err != nil {
		return ""
	}

	// First, try to get identifier directly from field 2 of level 1
	// (this is the case for list styles like "text-1-liststyle-Bullet")
	if identifier, ok := subMsg.GetString(2); ok && identifier != "" {
		return identifier
	}

	// If not found, continue with the old logic for other style types
	// Get field 3 from level 1 (this is another protobuf-encoded sub-message)
	b2, ok := subMsg.GetBytes(3)
	if !ok {
		return ""
	}

	// Parse as protobuf message (level 2)
	subMsg2, err := base.ParseMessage(b2)
	if err != nil {
		return ""
	}

	// Get varint from field 1 of level 2 (this is the reference to the super style)
	ref, ok := subMsg2.GetVarint(1)
	if !ok {
		return ""
	}

	// Get the referenced super style object
	superObj := store.Object(ref)
	if superObj == nil {
		return ""
	}

	// Get field 1 from super object (this is a protobuf-encoded string with the identifier)
	b3, ok := superObj.Message.GetBytes(1)
	if !ok {
		return ""
	}

	// Parse as protobuf message (level 3)
	subMsg3, err := base.ParseMessage(b3)
	if err != nil {
		return ""
	}

	// Get identifier from field 2
	identifier, _ := subMsg3.GetString(2)
	return identifier
}

// extractNoteText extracts speaker notes from a slide.
func extractNoteText(store *tsp.ObjectStore, slideObj *tsp.IwaObject, hyperlinkMap map[string]string) string {
	// Get note reference
	noteRef, err := slideObj.Message.GetMessage(knSlideNote)
	if err != nil || noteRef == nil {
		return ""
	}

	noteID, ok := noteRef.GetVarint(1)
	if !ok {
		return ""
	}

	noteObj := store.Object(noteID)
	if noteObj == nil || noteObj.Type != typeKNNote {
		return ""
	}

	// Get contained_storage (StorageArchive)
	storageRef, err := noteObj.Message.GetMessage(knNoteContainedStorage)
	if err != nil || storageRef == nil {
		return ""
	}

	storageID, ok := storageRef.GetVarint(1)
	if !ok {
		return ""
	}

	storageObj := store.Object(storageID)
	if storageObj == nil {
		return ""
	}

	// Get text
	texts := storageObj.Message.GetStrings(knStorageText)
	if len(texts) == 0 {
		return ""
	}

	text := texts[0]
	if text == "" {
		return ""
	}

	// Apply hyperlinks first (before character formatting to preserve positions)
	text = applyHyperlinks(store, storageObj, text, hyperlinkMap)

	// Apply character formatting (bold, italic) after hyperlinks
	charStyleTable, _ := storageObj.Message.GetMessage(8)
	if charStyleTable != nil {
		text = applyDrawableCharacterFormatting(store, text, charStyleTable)
	}

	return text
}
