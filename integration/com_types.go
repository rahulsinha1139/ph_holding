package integration

// ExcelWorkbookData represents data to insert into Excel
type ExcelWorkbookData struct {
	WorkbookPath string
	SheetName    string
	StartCell    string // e.g., "A2"
	Data         [][]interface{}
	MacroName    string // Optional macro to run after insertion
}

// WordDocumentData represents Word document generation
type WordDocumentData struct {
	TemplatePath string
	OutputPath   string
	Replacements map[string]string // Placeholder -> Value
}
