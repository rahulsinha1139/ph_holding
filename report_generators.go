package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

// =============================================================================
// REPORT GENERATION HANDLERS (PDF + EXCEL)
// =============================================================================
//
// PURPOSE:
// - Generate professional PDF reports with charts
// - Generate Excel reports with data tables and formulas
// - Support all report categories: sales, customers, operations, financial
//
// LIBRARIES:
// - gofpdf: Pure Go PDF generation (no Pandoc dependency!)
// - excelize: Excel XLSX generation with full formatting
//
// BUSINESS CONTEXT:
// - Reports are survival metrics (runway, collections, margins)
// - Must be audit-trail compliant (timestamp, metadata)
// - Export to accountants/investors (professional formatting)
// =============================================================================

// RegisterReportHandlers registers all report generation job handlers
func (a *App) RegisterReportHandlers() {
	a.jobQueue.RegisterHandler("report_generate", a.handleReportGenerate)
}

// handleReportGenerate processes report generation jobs
func (a *App) handleReportGenerate(ctx context.Context, job *Job) error {
	var input ReportGenerateInput
	if err := json.Unmarshal([]byte(job.Input), &input); err != nil {
		return fmt.Errorf("invalid job input: %w", err)
	}

	// Update progress
	job.Progress = 10
	if err := a.jobQueue.updateJob(job); err != nil {
		return fmt.Errorf("failed to persist report job progress: %w", err)
	}

	// Get report data
	dateRange := calculateDateRange(input.DateRange.Start, input.DateRange.End)
	reportData, err := a.GetReportData(input.Category, dateRange)
	if err != nil {
		return fmt.Errorf("failed to fetch report data: %w", err)
	}

	job.Progress = 30
	if err := a.jobQueue.updateJob(job); err != nil {
		return fmt.Errorf("failed to persist report job progress: %w", err)
	}

	// Generate file based on format
	var filePath string
	// Convert DateRange to expected format
	dateRangeStruct := struct{ Start, End string }{
		Start: input.DateRange.Start,
		End:   input.DateRange.End,
	}
	switch input.Format {
	case "pdf":
		filePath, err = a.generatePDFReport(input.Category, reportData, dateRangeStruct)
	case "excel":
		filePath, err = a.generateExcelReport(input.Category, reportData, dateRangeStruct)
	case "csv":
		filePath, err = a.generateCSVReport(input.Category, reportData, dateRangeStruct)
	default:
		return fmt.Errorf("unsupported format: %s", input.Format)
	}

	if err != nil {
		return fmt.Errorf("report generation failed: %w", err)
	}

	job.Progress = 90
	if err := a.jobQueue.updateJob(job); err != nil {
		return fmt.Errorf("failed to persist report job progress: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat generated file: %w", err)
	}

	// Prepare output
	output := ReportGenerateOutput{
		FilePath:  filePath,
		FileSize:  fileInfo.Size(),
		Generated: time.Now(),
	}

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	job.Output = string(outputJSON)
	return nil
}

// =============================================================================
// PDF REPORT GENERATION
// =============================================================================

func (a *App) generatePDFReport(category string, data ReportData, dateRange struct{ Start, End string }) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(0, 10, fmt.Sprintf("PH Holdings - %s Report", titleCase(category)))
	pdf.Ln(12)

	// Metadata
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.Cell(0, 6, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Date Range: %s to %s", dateRange.Start, dateRange.End))
	pdf.Ln(10)

	pdf.SetTextColor(0, 0, 0)

	// Category-specific content
	switch category {
	case "sales":
		a.addSalesReportToPDF(pdf, data)
	case "customers":
		a.addCustomersReportToPDF(pdf, data)
	case "operations":
		a.addOperationsReportToPDF(pdf, data)
	case "inventory":
		a.addInventoryReportToPDF(pdf, data)
	case "financial":
		a.addFinancialReportToPDF(pdf, data)
	default:
		return "", fmt.Errorf("unknown report category: %s", category)
	}

	// Footer
	pdf.SetY(-15)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(150, 150, 150)
	pdf.CellFormat(0, 10, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")

	// Save to file
	exportDir := a.getExportDir("report", "", "", time.Now().Year())

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_report_%s.pdf", category, timestamp)
	filePath := filepath.Join(exportDir, filename)

	if err := pdf.OutputFileAndClose(filePath); err != nil {
		return "", fmt.Errorf("failed to save PDF: %w", err)
	}

	return filePath, nil
}

func (a *App) addSalesReportToPDF(pdf *gofpdf.Fpdf, data ReportData) {
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Key Metrics")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(70, 7, "Win Rate:")
	pdf.Cell(0, 7, fmt.Sprintf("%.1f%%", data.WinRate*100))
	pdf.Ln(7)

	pdf.Cell(70, 7, "Conversion Rate:")
	pdf.Cell(0, 7, fmt.Sprintf("%.1f%%", data.ConversionRate*100))
	pdf.Ln(7)

	pdf.Cell(70, 7, "Average Deal Size:")
	pdf.Cell(0, 7, fmt.Sprintf("%.2f BHD", data.AvgDealSize))
	pdf.Ln(12)

	// Pipeline table
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Pipeline by Stage")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(60, 7, "Stage", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 7, "Count", "1", 0, "C", false, 0, "")
	pdf.CellFormat(50, 7, "Value (BHD)", "1", 0, "R", false, 0, "")
	pdf.Ln(7)

	pdf.SetFont("Arial", "", 10)
	for _, stage := range data.Pipeline {
		pdf.CellFormat(60, 7, stage.Stage, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 7, fmt.Sprintf("%d", stage.Count), "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 7, fmt.Sprintf("%.2f", stage.Value), "1", 0, "R", false, 0, "")
		pdf.Ln(7)
	}
}

func (a *App) addCustomersReportToPDF(pdf *gofpdf.Fpdf, data ReportData) {
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Customer Metrics")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(80, 7, "Average Payment Days:")
	pdf.Cell(0, 7, fmt.Sprintf("%.0f days", data.AvgPaymentDays))
	pdf.Ln(7)

	pdf.Cell(80, 7, "Collection Efficiency:")
	pdf.Cell(0, 7, fmt.Sprintf("%.1f%%", data.CollectionEff*100))
	pdf.Ln(12)

	// Grade distribution
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Grade Distribution")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(40, 7, "Grade", "1", 0, "C", false, 0, "")
	pdf.CellFormat(50, 7, "Customers", "1", 0, "C", false, 0, "")
	pdf.CellFormat(50, 7, "Percentage", "1", 0, "C", false, 0, "")
	pdf.Ln(7)

	pdf.SetFont("Arial", "", 10)
	for _, grade := range data.GradeDistribution {
		pdf.CellFormat(40, 7, grade.Grade, "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 7, fmt.Sprintf("%d", grade.Count), "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 7, fmt.Sprintf("%.1f%%", grade.Percentage), "1", 0, "C", false, 0, "")
		pdf.Ln(7)
	}
}

func (a *App) addOperationsReportToPDF(pdf *gofpdf.Fpdf, data ReportData) {
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Operations Metrics")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(80, 7, "Average Lead Time:")
	pdf.Cell(0, 7, fmt.Sprintf("%d days", data.AvgLeadTime))
	pdf.Ln(7)

	pdf.Cell(80, 7, "On-Time Delivery:")
	pdf.Cell(0, 7, fmt.Sprintf("%.1f%%", data.OnTimeDelivery*100))
	pdf.Ln(7)

	pdf.Cell(80, 7, "Pending Shipments:")
	pdf.Cell(0, 7, fmt.Sprintf("%d", data.PendingShipments))
	pdf.Ln(12)
}

func (a *App) addInventoryReportToPDF(pdf *gofpdf.Fpdf, data ReportData) {
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Inventory Metrics")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(70, 7, "Total Items:")
	pdf.Cell(0, 7, fmt.Sprintf("%d", data.TotalItems))
	pdf.Ln(7)

	pdf.Cell(70, 7, "Total Value:")
	pdf.Cell(0, 7, fmt.Sprintf("%.2f BHD", data.TotalValue))
	pdf.Ln(7)

	pdf.Cell(70, 7, "Low Stock Alerts:")
	pdf.Cell(0, 7, fmt.Sprintf("%d", data.LowStockAlerts))
	pdf.Ln(12)
}

func (a *App) addFinancialReportToPDF(pdf *gofpdf.Fpdf, data ReportData) {
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Financial Health")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(70, 7, "Cash Balance:")
	pdf.Cell(0, 7, fmt.Sprintf("%.2f BHD", data.CashBalance))
	pdf.Ln(7)

	pdf.Cell(70, 7, "Monthly Burn:")
	pdf.Cell(0, 7, fmt.Sprintf("%.2f BHD", data.MonthlyBurn))
	pdf.Ln(7)

	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(70, 7, "Cash Runway:")

	// Color-code runway (red if critical)
	if data.RunwayMonths < 3 {
		pdf.SetTextColor(200, 0, 0)
	} else {
		pdf.SetTextColor(0, 150, 0)
	}
	pdf.Cell(0, 7, fmt.Sprintf("%.1f months", data.RunwayMonths))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// MRR
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Monthly Recurring Revenue")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(70, 7, "Current MRR:")
	pdf.Cell(0, 7, fmt.Sprintf("%.2f BHD", data.MRR))
	pdf.Ln(7)

	pdf.Cell(70, 7, "MRR Target:")
	pdf.Cell(0, 7, fmt.Sprintf("%.2f BHD", data.MRRTarget))
	pdf.Ln(7)

	sustainabilityRatio := (data.MRR / data.MonthlyBurn) * 100
	pdf.Cell(70, 7, "Sustainability Ratio:")
	pdf.Cell(0, 7, fmt.Sprintf("%.0f%% (Target: 100%%)", sustainabilityRatio))
	pdf.Ln(12)

	// Overdue receivables
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Overdue Receivables")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(70, 7, "Aging", "1", 0, "L", false, 0, "")
	pdf.CellFormat(60, 7, "Amount (BHD)", "1", 0, "R", false, 0, "")
	pdf.Ln(7)

	pdf.SetFont("Arial", "", 10)
	totalOverdue := 0.0
	for _, bucket := range data.Overdue {
		pdf.CellFormat(70, 7, bucket.Days, "1", 0, "L", false, 0, "")
		pdf.CellFormat(60, 7, fmt.Sprintf("%.2f", bucket.Amount), "1", 0, "R", false, 0, "")
		pdf.Ln(7)
		totalOverdue += bucket.Amount
	}

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(70, 7, "TOTAL", "1", 0, "L", false, 0, "")
	pdf.CellFormat(60, 7, fmt.Sprintf("%.2f", totalOverdue), "1", 0, "R", false, 0, "")
}

// =============================================================================
// EXCEL REPORT GENERATION
// =============================================================================

func (a *App) generateExcelReport(category string, data ReportData, dateRange struct{ Start, End string }) (string, error) {
	f := excelize.NewFile()
	defer f.Close()
	xl := newGuardedExcelFile(f)

	sheetName := "Report"
	index := xl.NewSheet(sheetName)
	f.SetActiveSheet(index)

	// Title
	xl.SetCellValue(sheetName, "A1", fmt.Sprintf("PH Holdings - %s Report", titleCase(category)))
	titleStyle := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16},
	})
	xl.SetCellStyle(sheetName, "A1", "A1", titleStyle)

	// Metadata
	xl.SetCellValue(sheetName, "A2", fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	xl.SetCellValue(sheetName, "A3", fmt.Sprintf("Date Range: %s to %s", dateRange.Start, dateRange.End))

	// Category-specific content
	switch category {
	case "sales":
		a.addSalesReportToExcel(xl, sheetName, data)
	case "customers":
		a.addCustomersReportToExcel(xl, sheetName, data)
	case "operations":
		a.addOperationsReportToExcel(xl, sheetName, data)
	case "inventory":
		a.addInventoryReportToExcel(xl, sheetName, data)
	case "financial":
		a.addFinancialReportToExcel(xl, sheetName, data)
	default:
		return "", fmt.Errorf("unknown report category: %s", category)
	}

	if err := xl.Err(); err != nil {
		return "", fmt.Errorf("failed to populate Excel report: %w", err)
	}

	// Save to file
	exportDir := a.getExportDir("report", "", "", time.Now().Year())

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_report_%s.xlsx", category, timestamp)
	filePath := filepath.Join(exportDir, filename)

	xl.SaveAs(filePath)
	if err := xl.Err(); err != nil {
		return "", fmt.Errorf("failed to save Excel report: %w", err)
	}

	return filePath, nil
}

func (a *App) addSalesReportToExcel(xl *guardedExcelFile, sheet string, data ReportData) {
	// Headers
	headerStyle := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	// Metrics
	xl.SetCellValue(sheet, "A5", "Key Metrics")
	xl.SetCellStyle(sheet, "A5", "A5", headerStyle)
	xl.SetCellValue(sheet, "A6", "Win Rate")
	xl.SetCellValue(sheet, "B6", data.WinRate)
	xl.SetCellValue(sheet, "A7", "Conversion Rate")
	xl.SetCellValue(sheet, "B7", data.ConversionRate)
	xl.SetCellValue(sheet, "A8", "Avg Deal Size (BHD)")
	xl.SetCellValue(sheet, "B8", data.AvgDealSize)

	// Format percentages
	xl.SetCellStyle(sheet, "B6", "B7", getPercentStyle(xl))
	xl.SetCellStyle(sheet, "B8", "B8", getCurrencyStyle(xl))

	// Pipeline table
	row := 10
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Pipeline by Stage")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "A"+fmt.Sprint(row), headerStyle)
	row++

	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Stage")
	xl.SetCellValue(sheet, "B"+fmt.Sprint(row), "Count")
	xl.SetCellValue(sheet, "C"+fmt.Sprint(row), "Value (BHD)")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "C"+fmt.Sprint(row), headerStyle)
	row++

	for _, stage := range data.Pipeline {
		xl.SetCellValue(sheet, "A"+fmt.Sprint(row), stage.Stage)
		xl.SetCellValue(sheet, "B"+fmt.Sprint(row), stage.Count)
		xl.SetCellValue(sheet, "C"+fmt.Sprint(row), stage.Value)
		xl.SetCellStyle(sheet, "C"+fmt.Sprint(row), "C"+fmt.Sprint(row), getCurrencyStyle(xl))
		row++
	}
}

func (a *App) addCustomersReportToExcel(xl *guardedExcelFile, sheet string, data ReportData) {
	headerStyle := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	xl.SetCellValue(sheet, "A5", "Customer Metrics")
	xl.SetCellStyle(sheet, "A5", "A5", headerStyle)
	xl.SetCellValue(sheet, "A6", "Avg Payment Days")
	xl.SetCellValue(sheet, "B6", data.AvgPaymentDays)
	xl.SetCellValue(sheet, "A7", "Collection Efficiency")
	xl.SetCellValue(sheet, "B7", data.CollectionEff)
	xl.SetCellStyle(sheet, "B7", "B7", getPercentStyle(xl))

	row := 9
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Grade Distribution")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "A"+fmt.Sprint(row), headerStyle)
	row++

	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Grade")
	xl.SetCellValue(sheet, "B"+fmt.Sprint(row), "Customers")
	xl.SetCellValue(sheet, "C"+fmt.Sprint(row), "Percentage")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "C"+fmt.Sprint(row), headerStyle)
	row++

	for _, grade := range data.GradeDistribution {
		xl.SetCellValue(sheet, "A"+fmt.Sprint(row), grade.Grade)
		xl.SetCellValue(sheet, "B"+fmt.Sprint(row), grade.Count)
		xl.SetCellValue(sheet, "C"+fmt.Sprint(row), grade.Percentage/100)
		xl.SetCellStyle(sheet, "C"+fmt.Sprint(row), "C"+fmt.Sprint(row), getPercentStyle(xl))
		row++
	}
}

func (a *App) addOperationsReportToExcel(xl *guardedExcelFile, sheet string, data ReportData) {
	headerStyle := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	xl.SetCellValue(sheet, "A5", "Operations Metrics")
	xl.SetCellStyle(sheet, "A5", "A5", headerStyle)
	xl.SetCellValue(sheet, "A6", "Avg Lead Time (days)")
	xl.SetCellValue(sheet, "B6", data.AvgLeadTime)
	xl.SetCellValue(sheet, "A7", "On-Time Delivery")
	xl.SetCellValue(sheet, "B7", data.OnTimeDelivery)
	xl.SetCellStyle(sheet, "B7", "B7", getPercentStyle(xl))
	xl.SetCellValue(sheet, "A8", "Pending Shipments")
	xl.SetCellValue(sheet, "B8", data.PendingShipments)
}

func (a *App) addInventoryReportToExcel(xl *guardedExcelFile, sheet string, data ReportData) {
	headerStyle := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	xl.SetCellValue(sheet, "A5", "Inventory Metrics")
	xl.SetCellStyle(sheet, "A5", "A5", headerStyle)
	xl.SetCellValue(sheet, "A6", "Total Items")
	xl.SetCellValue(sheet, "B6", data.TotalItems)
	xl.SetCellValue(sheet, "A7", "Total Value (BHD)")
	xl.SetCellValue(sheet, "B7", data.TotalValue)
	xl.SetCellStyle(sheet, "B7", "B7", getCurrencyStyle(xl))
	xl.SetCellValue(sheet, "A8", "Low Stock Alerts")
	xl.SetCellValue(sheet, "B8", data.LowStockAlerts)
}

func (a *App) addFinancialReportToExcel(xl *guardedExcelFile, sheet string, data ReportData) {
	headerStyle := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	xl.SetCellValue(sheet, "A5", "Financial Health")
	xl.SetCellStyle(sheet, "A5", "A5", headerStyle)
	xl.SetCellValue(sheet, "A6", "Cash Balance (BHD)")
	xl.SetCellValue(sheet, "B6", data.CashBalance)
	xl.SetCellValue(sheet, "A7", "Monthly Burn (BHD)")
	xl.SetCellValue(sheet, "B7", data.MonthlyBurn)
	xl.SetCellValue(sheet, "A8", "Cash Runway (months)")
	xl.SetCellValue(sheet, "B8", data.RunwayMonths)
	xl.SetCellStyle(sheet, "B6", "B7", getCurrencyStyle(xl))

	// MRR
	row := 10
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Monthly Recurring Revenue")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "A"+fmt.Sprint(row), headerStyle)
	row++
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Current MRR (BHD)")
	xl.SetCellValue(sheet, "B"+fmt.Sprint(row), data.MRR)
	xl.SetCellStyle(sheet, "B"+fmt.Sprint(row), "B"+fmt.Sprint(row), getCurrencyStyle(xl))
	row++
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "MRR Target (BHD)")
	xl.SetCellValue(sheet, "B"+fmt.Sprint(row), data.MRRTarget)
	xl.SetCellStyle(sheet, "B"+fmt.Sprint(row), "B"+fmt.Sprint(row), getCurrencyStyle(xl))
	row += 2

	// Overdue receivables
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Overdue Receivables")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "A"+fmt.Sprint(row), headerStyle)
	row++
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "Aging")
	xl.SetCellValue(sheet, "B"+fmt.Sprint(row), "Amount (BHD)")
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "B"+fmt.Sprint(row), headerStyle)
	row++

	for _, bucket := range data.Overdue {
		xl.SetCellValue(sheet, "A"+fmt.Sprint(row), bucket.Days)
		xl.SetCellValue(sheet, "B"+fmt.Sprint(row), bucket.Amount)
		xl.SetCellStyle(sheet, "B"+fmt.Sprint(row), "B"+fmt.Sprint(row), getCurrencyStyle(xl))
		row++
	}

	// Total formula
	xl.SetCellValue(sheet, "A"+fmt.Sprint(row), "TOTAL")
	startRow := row - len(data.Overdue)
	endRow := row - 1
	xl.SetCellFormula(sheet, "B"+fmt.Sprint(row), fmt.Sprintf("=SUM(B%d:B%d)", startRow, endRow))
	xl.SetCellStyle(sheet, "A"+fmt.Sprint(row), "B"+fmt.Sprint(row), headerStyle)
}

// =============================================================================
// CSV REPORT GENERATION (Reuse existing function)
// =============================================================================

func (a *App) generateCSVReport(category string, data ReportData, dateRange struct{ Start, End string }) (string, error) {
	exportDir := a.getExportDir("report", "", "", time.Now().Year())

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_report_%s.csv", category, timestamp)
	filePath := filepath.Join(exportDir, filename)

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal CSV report payload: %w", err)
	}
	return a.exportReportToCSV(filePath, category, string(dataJSON))
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func calculateDateRange(start, end string) string {
	startTime, _ := time.Parse("2006-01-02", start)
	endTime, _ := time.Parse("2006-01-02", end)
	daysDiff := int(endTime.Sub(startTime).Hours() / 24)

	if daysDiff <= 7 {
		return "week"
	} else if daysDiff <= 31 {
		return "month"
	} else if daysDiff <= 92 {
		return "quarter"
	}
	return "year"
}

func getPercentStyle(xl *guardedExcelFile) int {
	return xl.NewStyle(&excelize.Style{
		NumFmt: 10, // 0.00%
	})
}

func getCurrencyStyle(xl *guardedExcelFile) int {
	return xl.NewStyle(&excelize.Style{
		NumFmt:       164, // Custom format
		CustomNumFmt: stringPtr("#,##0.00"),
	})
}

func stringPtr(s string) *string {
	return &s
}
