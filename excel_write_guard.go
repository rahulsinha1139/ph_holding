package main

import "github.com/xuri/excelize/v2"

// guardedExcelFile captures the first workbook mutation error so callers can
// keep layout code readable while still failing the export if a write breaks.
type guardedExcelFile struct {
	file *excelize.File
	err  error
}

func newGuardedExcelFile(file *excelize.File) *guardedExcelFile {
	return &guardedExcelFile{file: file}
}

func (g *guardedExcelFile) capture(err error) {
	if err != nil && g.err == nil {
		g.err = err
	}
}

func (g *guardedExcelFile) Err() error {
	return g.err
}

func (g *guardedExcelFile) File() *excelize.File {
	return g.file
}

func (g *guardedExcelFile) NewStyle(style *excelize.Style) int {
	id, err := g.file.NewStyle(style)
	g.capture(err)
	return id
}

func (g *guardedExcelFile) NewSheet(name string) int {
	idx, err := g.file.NewSheet(name)
	g.capture(err)
	return idx
}

func (g *guardedExcelFile) SetSheetName(source, target string) {
	g.capture(g.file.SetSheetName(source, target))
}

func (g *guardedExcelFile) SetColWidth(sheet, startCol, endCol string, width float64) {
	g.capture(g.file.SetColWidth(sheet, startCol, endCol, width))
}

func (g *guardedExcelFile) SetRowHeight(sheet string, row int, height float64) {
	g.capture(g.file.SetRowHeight(sheet, row, height))
}

func (g *guardedExcelFile) MergeCell(sheet, startCell, endCell string) {
	g.capture(g.file.MergeCell(sheet, startCell, endCell))
}

func (g *guardedExcelFile) SetCellValue(sheet, cell string, value interface{}) {
	g.capture(g.file.SetCellValue(sheet, cell, value))
}

func (g *guardedExcelFile) SetCellStyle(sheet, topLeftCell, bottomRightCell string, styleID int) {
	g.capture(g.file.SetCellStyle(sheet, topLeftCell, bottomRightCell, styleID))
}

func (g *guardedExcelFile) SetCellFormula(sheet, cell, formula string) {
	g.capture(g.file.SetCellFormula(sheet, cell, formula))
}

func (g *guardedExcelFile) AddDataValidation(sheet string, dv *excelize.DataValidation) {
	g.capture(g.file.AddDataValidation(sheet, dv))
}

func (g *guardedExcelFile) SaveAs(path string) {
	g.capture(g.file.SaveAs(path))
}

func (g *guardedExcelFile) GetSheetIndex(sheet string) int {
	idx, err := g.file.GetSheetIndex(sheet)
	g.capture(err)
	return idx
}
