# What Can Be Extracted from .pages Files

## Current Implementation ✅

### Main Document Text
- ✅ Text extraction from body storage
- ✅ Headings (Heading 1-6 → `#`, `##`, ...)
- ✅ Bulleted lists (`-`)
- ✅ Bold text (`**bold**`)
- ✅ Italic (`*italic*`)
- ✅ Hyperlinks (`[text](url)`)
- ✅ Tables (extracted and converted to markdown tables)
  - ✅ Plain text cells
  - ✅ Number cells
  - ✅ Boolean cells
  - ⚠️ Rich text cells (displayed as "[rich text]" - requires additional implementation)

## What Else Is in .pages Files 🔍

### 1. Rich Text in Table Cells
**Complexity:** Medium ⚠️

Rich text cells reference a separate text storage that needs to be extracted:
- DataStore field 17: rich_text_table reference
- TST.RichTextPayloadArchive contains references to TSWP.StorageArchive
- Need to extract text and apply formatting

**Current Status:** Rich text cells are detected but displayed as "[rich text]" placeholder.
// Returns [][]string - array of tables
```

### 2. Sections (TP.SectionArchive, type 10011)
**Complexity:** Medium

Structure:
- TP.SectionArchive contains section settings
- References to page masters (first, even, odd)
- Header/footer settings

**What Can Be Extracted:**
- Section names
- Page settings (size, orientation)
- Document division into sections in markdown

### 3. Headers/Footers
**Complexity:** Low

Structure:
- TSWP.StorageArchive with kind=1 (HEADER) or kind=2 (FOOTER)
- Bound to sections via TP.SectionTemplate

**What Can Be Extracted:**
- Header and footer text
- Page numbers

### 4. Text Boxes
**Complexity:** Medium

Structure:
- TSWP.ShapeInfoArchive (type 2011)
- Reference to TSWP.StorageArchive with kind=3 (TEXTBOX)

**What Can Be Extracted:**
- Text from floating text boxes
- Position on page

### 5. Images (TSD.ImageArchive, type 3005)
**Complexity:** Medium

Structure:
- TSD.ImageArchive contains references to data
- Data/ directory in ZIP archive contains binary data

**What Can Be Extracted:**
- Image references
- Image extraction to separate files
- Insertion into markdown as `![alt](image.png)`

### 6. Footnotes
**Complexity:** Medium

Structure:
- TSWP.StorageArchive with kind=2 (FOOTNOTE)
- Bound to text via table_footnote

**What Can Be Extracted:**
- Footnote text
- Footnote markers in text

### 7. Comments
**Complexity:** Low

Structure:
- TSWP.CommentInfoArchive
- Bound to text ranges

**What Can Be Extracted:**
- Comment text
- Binding to document

### 8. Bookmarks
**Complexity:** Low

Structure:
- TSWP.BookmarkFieldArchive (type 2035)
- Bound to text ranges

**What Can Be Extracted:**
- Bookmark names
- Positions in document

### 9. Table of Contents
**Complexity:** Medium

Structure:
- TSWP.TOCInfoArchive
- Automatically generated from headings

**What Can Be Extracted:**
- Table of contents structure
- Links to sections

## Implementation Priorities

### High Priority 🎯
1. **Tables** - the most important feature after text
   - Complexity: ~500 lines of code
   - Benefit: very high

2. **Images** - important part of documents
   - Complexity: ~200 lines of code
   - Benefit: high

### Medium Priority 📊
3. **Headers/Footers** - useful information
   - Complexity: ~100 lines of code
   - Benefit: medium

4. **Text Boxes** - frequently used
   - Complexity: ~150 lines of code
   - Benefit: medium

### Low Priority 📝
5. **Footnotes** - rarely used
6. **Comments** - meta-information
7. **Bookmarks** - navigation
8. **Table of Contents** - can be generated from headings

## Usage Examples

### Tables
```go
// Extract all tables
tables, err := iwaread.ReadTables("document.pages")
for i, table := range tables {
    fmt.Printf("Table %d:\n", i+1)
    for _, row := range table {
        fmt.Printf("  %v\n", row)
    }
}
```

### Images
```go
// Extract images
images, err := iwaread.ReadImages("document.pages")
for _, img := range images {
    os.WriteFile(img.Filename, img.Data, 0644)
}
```

### Full Extraction
```go
// Extract everything
doc, err := iwaread.ReadFull("document.pages")
fmt.Println(doc.Markdown)      // Main text
fmt.Println(doc.Tables)        // Tables
fmt.Println(doc.Images)        // Images
fmt.Println(doc.Headers)       // Headers
fmt.Println(doc.TextBoxes)     // Text boxes
```

## Technical Details

### Table Implementation Challenges

1. **Binary Cell Format**
   - 12-byte header with flags
   - Variable-length optional fields
   - Decimal128 for numbers (complex format)

2. **String Table**
   - Strings stored separately in DataStore
   - Cells reference strings by ID
   - Need to decode TST.DataList

3. **Tile-based Storage**
   - Data split into tiles
   - Each tile contains several rows
   - Complex indexing

4. **Formulas**
   - Cells can contain formulas
   - Need to extract from TSCE (calculation engine)
   - Complex dependency structure

### Implementation Time Estimate

- Tables: 2-3 days
- Images: 1 day
- Headers/Footers: 0.5 days
- Text Boxes: 1 day
- Footnotes: 0.5 days
- Everything else: 1-2 days

**Total:** ~7-10 days for full implementation
