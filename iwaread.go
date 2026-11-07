package iwaread

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/wmentor/iwaread/internal/base"
	"github.com/wmentor/iwaread/internal/tsp"
)

// Type IDs
const (
	typeTPDocument     = 10000
	typeStorage        = 2001
	typeHyperlinkField = 2032
)

// Storage field numbers
const (
	storageKind            = 1
	storageText            = 3
	storageTableParaStyle  = 5
	storageTableListStyle  = 7
	storageTableCharStyle  = 8
	storageTableSmartField = 11
)

// TPDocument field numbers
const (
	tpDocBodyStorage = 4
)

// ObjectAttributeTable field numbers
const (
	attrTableEntries    = 1
	entryCharacterIndex = 1
	entryObject         = 2
)

// StyleArchive field numbers
const (
	styleName       = 1
	styleIdentifier = 2
)

// ParagraphStyleArchive field numbers
const (
	paraStyleSuper     = 1
	paraStyleCharProps = 11
)

// CharacterStyleArchive field numbers
const (
	charStyleCharProps = 11
)

// CharacterStylePropertiesArchive field numbers
const (
	charPropsBold   = 1
	charPropsItalic = 2
)

// ParagraphStylePropertiesArchive field numbers
const (
	paraPropsOutlineLevel = 27
)

// HyperlinkFieldArchive field numbers
const (
	hyperlinkURL = 2
)

// ReadPages reads a .pages file and returns its content as markdown.
func ReadPages(filename string) (string, error) {
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

	return extractPagesMarkdown(store)
}

func extractPagesMarkdown(store *tsp.ObjectStore) (string, error) {
	// Find TP.DocumentArchive
	docObj := store.FindByType(typeTPDocument)
	if docObj == nil {
		return "", fmt.Errorf("TP.DocumentArchive not found — not a Pages document?")
	}

	// Get body storage reference
	bodyID, ok := tsp.RefID(docObj.Message, tpDocBodyStorage)
	if !ok {
		return "", nil // Page-layout document with no body
	}

	bodyObj := store.Object(bodyID)
	if bodyObj == nil {
		return "", fmt.Errorf("body storage not found")
	}

	// Extract text
	text := extractText(bodyObj.Message)
	if text == "" {
		return "", nil
	}

	// Convert to markdown with styling
	md := convertToMarkdown(store, bodyObj.Message, text)
	return md, nil
}

func extractText(msg *base.RawMessage) string {
	texts := msg.GetStrings(storageText)
	if len(texts) == 0 {
		return ""
	}
	return texts[0]
}

func convertToMarkdown(store *tsp.ObjectStore, storageMsg *base.RawMessage, text string) string {
	var sb strings.Builder

	// Get tables
	paraStyleTable, _ := storageMsg.GetMessage(storageTableParaStyle)
	charStyleTable, _ := storageMsg.GetMessage(storageTableCharStyle)
	listStyleTable, _ := storageMsg.GetMessage(storageTableListStyle)
	smartFieldTable, _ := storageMsg.GetMessage(storageTableSmartField)

	// Extract tables with their positions (in runes)
	tables := extractTables(store, storageMsg)

	// Build a map of rune positions to tables
	tableMap := make(map[int]*Table)
	for i := range tables {
		tableMap[tables[i].Position] = &tables[i]
	}

	// Build paragraph boundaries
	paragraphs := splitParagraphs(text)

	// Process each paragraph
	for i, para := range paragraphs {
		if para.text == "" {
			sb.WriteString("\n")
			continue
		}

		// Get paragraph style
		paraStyleID := getEffectiveObject(paraStyleTable, para.start)
		paraStyleIdentifier := getStyleIdentifier(store, paraStyleID)

		// Get list style
		listStyleID := getEffectiveObject(listStyleTable, para.start)
		listStyleIdentifier := getStyleIdentifier(store, listStyleID)

		// Check if this is a list (identifier does not contain "None")
		// Note: identifier can be empty for dynamic list styles, which is still a list
		isList := listStyleID != 0 && !strings.Contains(listStyleIdentifier, "None")

		// Apply markdown formatting
		if isList {
			sb.WriteString("- ")
		} else if isHeading(paraStyleIdentifier) {
			level := getHeadingLevel(paraStyleIdentifier)
			sb.WriteString(strings.Repeat("#", level))
			sb.WriteString(" ")
		}

		// Process text with character formatting and table insertion
		// Convert para.start from byte offset to rune offset
		runeOffset := len([]rune(text[:para.start]))
		formattedText := formatTextRunWithTables(store, charStyleTable, smartFieldTable, para.text, runeOffset, tableMap)
		sb.WriteString(formattedText)

		if i < len(paragraphs)-1 {
			sb.WriteString("\n")
			if !isList && !isHeading(paraStyleIdentifier) {
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

type paragraph struct {
	start int
	text  string
}

func splitParagraphs(text string) []paragraph {
	var result []paragraph
	start := 0
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\n' || ch == 0x04 || ch == 0x05 || ch == 0x0c {
			result = append(result, paragraph{start: start, text: text[start:i]})
			start = i + 1
		}
	}
	if start < len(text) {
		result = append(result, paragraph{start: start, text: text[start:]})
	}
	return result
}

func getEffectiveObject(table *base.RawMessage, pos int) uint64 {
	if table == nil {
		return 0
	}
	entries, err := table.GetMessages(attrTableEntries)
	if err != nil {
		return 0
	}
	var value uint64
	for _, entry := range entries {
		idx, _ := entry.GetUint(entryCharacterIndex)
		if idx > pos {
			break
		}
		objID, ok := tsp.RefID(entry, entryObject)
		if ok {
			value = objID
		}
	}
	return value
}

func getStyleName(store *tsp.ObjectStore, styleID uint64) string {
	if styleID == 0 {
		return ""
	}
	obj := store.Object(styleID)
	if obj == nil {
		return ""
	}
	// Style has super at field 1, which has name at field 1
	super, err := obj.Message.GetMessage(paraStyleSuper)
	if err != nil || super == nil {
		return ""
	}
	name, _ := super.GetString(styleName)
	return name
}

func getStyleIdentifier(store *tsp.ObjectStore, styleID uint64) string {
	if styleID == 0 {
		return ""
	}
	obj := store.Object(styleID)
	if obj == nil {
		return ""
	}
	// Style has super at field 1, which has identifier at field 2
	super, err := obj.Message.GetMessage(paraStyleSuper)
	if err != nil || super == nil {
		return ""
	}
	identifier, _ := super.GetString(styleIdentifier)
	return identifier
}

func isHeading(styleIdentifier string) bool {
	// Universal identifier-based check
	// Examples: "text-0-paragraphstyle-Title", "text-11-paragraphstyle-Heading 1"
	lower := strings.ToLower(styleIdentifier)
	return strings.Contains(lower, "paragraphstyle-title") ||
		strings.Contains(lower, "paragraphstyle-heading")
}

func getHeadingLevel(styleIdentifier string) int {
	// Extract level from identifier like "text-11-paragraphstyle-Heading 2"
	lower := strings.ToLower(styleIdentifier)

	// Title -> level 1
	if strings.Contains(lower, "paragraphstyle-title") {
		return 1
	}

	// Heading N -> level N
	if strings.Contains(lower, "paragraphstyle-heading") {
		// Find "heading" and extract the number after it
		idx := strings.Index(lower, "heading")
		if idx != -1 {
			rest := lower[idx+7:] // skip "heading"
			// Skip spaces
			rest = strings.TrimLeft(rest, " ")
			// Extract first digit
			if len(rest) > 0 && rest[0] >= '1' && rest[0] <= '6' {
				return int(rest[0] - '0')
			}
		}
	}

	return 1
}

func formatTextRunWithTables(store *tsp.ObjectStore, charStyleTable, smartFieldTable *base.RawMessage, text string, baseRuneOffset int, tableMap map[int]*Table) string {
	if text == "" {
		return ""
	}

	var sb strings.Builder
	runStart := 0
	i := 0
	runePos := baseRuneOffset // Track rune position

	for i < len(text) {
		// Check for OBJECT REPLACEMENT CHARACTER (table placeholder)
		// U+FFFC is encoded as 3 bytes in UTF-8: 0xEF 0xBF 0xBC
		if i+2 < len(text) && text[i] == 0xEF && text[i+1] == 0xBF && text[i+2] == 0xBC {
			// Flush current run before table
			if runStart < i {
				runText := text[runStart:i]
				currentStyle := getEffectiveObject(charStyleTable, runePos-len([]rune(runText)))
				formatted := applyCharacterFormatting(store, currentStyle, runText)
				sb.WriteString(formatted)
			}

			// Check if there's a table at this rune position
			if table, ok := tableMap[runePos]; ok {
				// Insert table markdown
				sb.WriteString("\n\n")
				sb.WriteString(tableToMarkdown(table))
				sb.WriteString("\n")
			}

			i += 3    // Skip 3 bytes of U+FFFC
			runePos++ // U+FFFC is one rune
			runStart = i
			continue
		}

		// Check for hyperlink at current position
		var hyperlinkURL string
		if smartFieldTable != nil {
			hyperlinkURL = getHyperlinkURLAt(store, smartFieldTable, runePos)
		}

		if hyperlinkURL != "" {
			// Flush current run before hyperlink
			if runStart < i {
				runText := text[runStart:i]
				currentStyle := getEffectiveObject(charStyleTable, runePos-len([]rune(runText)))
				formatted := applyCharacterFormatting(store, currentStyle, runText)
				sb.WriteString(formatted)
			}

			// Find end of hyperlink run
			end := i
			endRunePos := runePos
			for end < len(text) {
				// Check if we hit another special character
				if end+2 < len(text) && text[end] == 0xEF && text[end+1] == 0xBF && text[end+2] == 0xBC {
					break
				}
				endURL := getHyperlinkURL(store, smartFieldTable, endRunePos)
				if endURL != hyperlinkURL {
					break
				}
				// Advance by one rune (handle multi-byte UTF-8)
				charLen := 1
				if end < len(text) {
					ch := text[end]
					if ch >= 0xC0 && ch < 0xE0 {
						charLen = 2
					} else if ch >= 0xE0 && ch < 0xF0 {
						charLen = 3
					} else if ch >= 0xF0 {
						charLen = 4
					}
				}
				end += charLen
				endRunePos++
			}

			// Write hyperlink
			sb.WriteString("[")
			sb.WriteString(text[i:end])
			sb.WriteString("](")
			sb.WriteString(normalizeHyperlinkURL(hyperlinkURL))
			sb.WriteString(")")

			i = end
			runePos = endRunePos
			runStart = end
			continue
		}

		// Determine byte length of current UTF-8 character
		charLen := 1
		if i < len(text) {
			ch := text[i]
			if ch >= 0xC0 && ch < 0xE0 {
				charLen = 2
			} else if ch >= 0xE0 && ch < 0xF0 {
				charLen = 3
			} else if ch >= 0xF0 {
				charLen = 4
			}
		}

		// Check character style change
		currentStyle := getEffectiveObject(charStyleTable, runePos)
		nextStyle := uint64(0)
		if i+charLen < len(text) {
			// Calculate next rune position
			nextRunePos := runePos + 1
			nextStyle = getEffectiveObject(charStyleTable, nextRunePos)
		}

		if currentStyle != nextStyle {
			// Flush current run
			runEnd := i + charLen
			runText := text[runStart:runEnd]
			if runText != "" {
				formatted := applyCharacterFormatting(store, currentStyle, runText)
				sb.WriteString(formatted)
			}
			runStart = runEnd
		}

		i += charLen
		runePos++
	}

	// Flush remaining text
	if runStart < len(text) {
		runText := text[runStart:]
		currentStyle := getEffectiveObject(charStyleTable, runePos-len([]rune(runText)))
		formatted := applyCharacterFormatting(store, currentStyle, runText)
		sb.WriteString(formatted)
	}

	return sb.String()
}

func formatTextRun(store *tsp.ObjectStore, charStyleTable, smartFieldTable *base.RawMessage, text string, baseOffset int) string {
	if text == "" {
		return ""
	}

	var sb strings.Builder
	runStart := 0
	i := 0

	for i < len(text) {
		pos := baseOffset + i

		// Check for hyperlink at current position
		var hyperlinkURL string
		if smartFieldTable != nil {
			hyperlinkURL = getHyperlinkURLAt(store, smartFieldTable, pos)
		}

		if hyperlinkURL != "" {
			// Flush current run before hyperlink
			if runStart < i {
				runText := text[runStart:i]
				currentStyle := getEffectiveObject(charStyleTable, baseOffset+runStart)
				formatted := applyCharacterFormatting(store, currentStyle, runText)
				sb.WriteString(formatted)
			}

			// Find end of hyperlink run
			end := i
			for end < len(text) {
				endURL := getHyperlinkURLAt(store, smartFieldTable, baseOffset+end)
				if endURL != hyperlinkURL {
					break
				}
				end++
			}

			// Write hyperlink
			sb.WriteString("[")
			sb.WriteString(text[i:end])
			sb.WriteString("](")
			sb.WriteString(normalizeHyperlinkURL(hyperlinkURL))
			sb.WriteString(")")

			i = end
			runStart = end
			continue
		}

		// Check character style change
		currentStyle := getEffectiveObject(charStyleTable, pos)
		nextStyle := uint64(0)
		if i+1 < len(text) {
			nextStyle = getEffectiveObject(charStyleTable, baseOffset+i+1)
		}

		if currentStyle != nextStyle {
			// Flush current run
			runEnd := i + 1
			runText := text[runStart:runEnd]
			if runText != "" {
				formatted := applyCharacterFormatting(store, currentStyle, runText)
				sb.WriteString(formatted)
			}
			runStart = runEnd
		}

		i++
	}

	// Flush remaining text
	if runStart < len(text) {
		runText := text[runStart:]
		currentStyle := getEffectiveObject(charStyleTable, baseOffset+runStart)
		formatted := applyCharacterFormatting(store, currentStyle, runText)
		sb.WriteString(formatted)
	}

	return sb.String()
}

func normalizeHyperlinkURL(rawURL string) string {
	// Check if this is a VS Code file+ URL
	if !strings.HasPrefix(rawURL, "https://file+.vscode-resource.vscode-cdn.net/") {
		return rawURL
	}

	// Extract the local path after the VS Code prefix
	localPath := strings.TrimPrefix(rawURL, "https://file+.vscode-resource.vscode-cdn.net")

	// URL-decode the path
	decoded, err := url.PathUnescape(localPath)
	if err != nil {
		decoded = localPath
	}

	// Find the repository root in the path
	// Look for pattern: /github.com/<owner>/<repo>/
	parts := strings.Split(decoded, "/")
	for i, part := range parts {
		if part == "github.com" && i+2 < len(parts) {
			// Found github.com, next two parts are owner and repo
			// Return everything after that
			if i+3 <= len(parts) {
				relativePath := strings.Join(parts[i+3:], "/")
				return relativePath
			}
		}
	}

	// If no github.com pattern found, return the original URL
	return rawURL
}

func getHyperlinkURL(store *tsp.ObjectStore, table *base.RawMessage, pos int) string {
	entries, err := table.GetMessages(attrTableEntries)
	if err != nil {
		return ""
	}
	var fieldID uint64
	for _, entry := range entries {
		idx, _ := entry.GetUint(entryCharacterIndex)
		if idx > pos {
			break
		}
		objID, ok := tsp.RefID(entry, entryObject)
		if ok {
			fieldID = objID
		} else {
			// Entry with no object reference marks the end of a smart field run
			fieldID = 0
		}
	}
	if fieldID == 0 {
		return ""
	}
	obj := store.Object(fieldID)
	if obj == nil || obj.Type != typeHyperlinkField {
		return ""
	}
	url, _ := obj.Message.GetString(hyperlinkURL)
	return url
}

func getHyperlinkURLAt(store *tsp.ObjectStore, table *base.RawMessage, pos int) string {
	entries, err := table.GetMessages(attrTableEntries)
	if err != nil {
		return ""
	}
	// Find entry that starts exactly at pos
	for _, entry := range entries {
		idx, _ := entry.GetUint(entryCharacterIndex)
		if idx == pos {
			objID, ok := tsp.RefID(entry, entryObject)
			if !ok {
				return ""
			}
			obj := store.Object(objID)
			if obj == nil || obj.Type != typeHyperlinkField {
				return ""
			}
			url, _ := obj.Message.GetString(hyperlinkURL)
			return url
		}
		if idx > pos {
			break
		}
	}
	return ""
}

func applyCharacterFormatting(store *tsp.ObjectStore, styleID uint64, text string) string {
	if styleID == 0 {
		return text
	}

	bold, italic := getCharacterFormatting(store, styleID)

	result := text
	if bold && italic {
		result = "***" + result + "***"
	} else if bold {
		result = "**" + result + "**"
	} else if italic {
		result = "*" + result + "*"
	}
	return result
}

func getCharacterFormatting(store *tsp.ObjectStore, styleID uint64) (bold, italic bool) {
	obj := store.Object(styleID)
	if obj == nil {
		return false, false
	}

	// Get char_properties from CharacterStyleArchive
	charProps, err := obj.Message.GetMessage(charStyleCharProps)
	if err != nil || charProps == nil {
		// Try ParagraphStyleArchive
		charProps, err = obj.Message.GetMessage(paraStyleCharProps)
		if err != nil || charProps == nil {
			return false, false
		}
	}

	bold, _ = charProps.GetBool(charPropsBold)
	italic, _ = charProps.GetBool(charPropsItalic)
	return bold, italic
}
