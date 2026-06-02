package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommercialExtractExportUsesProvidedFilteredRows(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	app := setupTestApp(t)
	rows := `[
		{"offer_number":"91-26","customer_name":"Sedres/Orbitus","total_value_bhd":2783,"stage":"Quoted","rfq_reference":"12345678","prepared_by":"Rahul"},
		{"offer_number":"SA-08-26","customer_name":"AHS Trading","total_value_bhd":1200,"stage":"Won","rfq_reference":"SA-08","prepared_by":"Peri"}
	]`

	path, err := app.ExportReport("offer_extract", "excel", rows)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(path, ".csv"))
	require.FileExists(t, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "PH Trading WLL - Offer Extract")
	require.Contains(t, text, "91-26")
	require.Contains(t, text, "Sedres/Orbitus")
	require.Contains(t, text, "SA-08-26")
}

func TestCommercialExtractExportSupportsPDF(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	app := setupTestApp(t)
	rows := `[{"offer_number":"91-26","customer_name":"Sedres/Orbitus","total_value_bhd":2783,"stage":"Quoted","rfq_reference":"12345678","prepared_by":"Rahul"}]`

	path, err := app.ExportReport("offer_extract", "pdf", rows)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(path, ".pdf"))
	require.FileExists(t, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(content), "%PDF"), "expected PDF file header")
}

func TestCommercialExtractExportRequiresWorkflowPermission(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	app := setupTestApp(t)
	rows := `[{"offer_number":"91-26","customer_name":"Sedres/Orbitus"}]`

	setFeedbackHardeningTestUser(app, "blocked-export", "staff", `[]`, "Blocked User")
	_, err := app.ExportReport("offer_extract", "excel", rows)
	require.Error(t, err)
	require.Contains(t, err.Error(), "offers:view")

	setFeedbackHardeningTestUser(app, "sales-export", "sales", `["offers:view"]`, "Sales User")
	path, err := app.ExportReport("offer_extract", "excel", rows)
	require.NoError(t, err)
	require.FileExists(t, path)
}

func TestFinancialReportExportRequiresFinancePermission(t *testing.T) {
	app := setupTestApp(t)

	setFeedbackHardeningTestUser(app, "reports-only", "manager", `["reports:view"]`, "Reports Only")
	_, err := app.ExportReport("financial", "csv", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "finance:view")
}

func TestFinancialReportExportGeneratesCSVForFinanceUser(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	app := setupTestApp(t)
	setFeedbackHardeningTestUser(app, "finance-export", "admin", `["finance:view","reports:view"]`, "Finance User")

	path, err := app.ExportReport("financial", "csv", "")
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(path, ".csv"))
	require.FileExists(t, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "PH Trading WLL - Financial Report")
	require.Contains(t, string(content), "Metric,Value")
}

func TestReportExportRejectsUnknownTypeBeforePathCreation(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	app := setupTestApp(t)
	setFeedbackHardeningTestUser(app, "reports-admin", "admin", `["reports:view"]`, "Reports Admin")

	_, err := app.ExportReport("../evil", "csv", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown report type")
}

func TestCommercialExtractSanitizesSpreadsheetFormulaCells(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	app := setupTestApp(t)
	setFeedbackHardeningTestUser(app, "sales-export", "sales", `["offers:view"]`, "Sales User")

	path, err := app.ExportReport("offer_extract", "excel", `[{"offer_number":"91-26","customer_name":"=cmd|' /C calc'!A0"}]`)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "'=cmd|' /C calc'!A0")
}

func TestSelectedBankStatementAccountMismatchIsRejected(t *testing.T) {
	selected := CompanyBankAccount{
		BankName:      "Al Salam Bank",
		AccountNumber: "216114100100",
		IBAN:          "BH67ALSABH000000216114100100",
	}

	err := validateParsedStatementMatchesSelectedAccount(&parsedStatement{
		AccountNumber: "99570076",
		IBAN:          "BH00NBOBBH00000099570076",
	}, selected)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match")

	err = validateParsedStatementMatchesSelectedAccount(&parsedStatement{
		AccountNumber: "216114100100",
	}, selected)
	require.NoError(t, err)

	err = validateParsedStatementMatchesSelectedAccount(&parsedStatement{
		AccountNumber: "000000216",
	}, selected)
	require.Error(t, err)

	err = validateParsedStatementMatchesSelectedAccount(&parsedStatement{
		AccountNumber: "114100100",
	}, selected)
	require.NoError(t, err)
}
