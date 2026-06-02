package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyDiscoveredFile_RecognizesCostWorkbookVariants(t *testing.T) {
	cases := []string{
		"PH Trading Costing MasterFile.xlsx",
		"COST 01-26_rev-1.xlsx",
		"Cost-06-26.xlsx",
		"cost_42-26.xls",
	}

	for _, name := range cases {
		if got := classifyDiscoveredFile(name); got != "costing_sheet" {
			t.Fatalf("expected %q to classify as costing_sheet, got %q", name, got)
		}
	}
}

func TestClassifyDiscoveredFileAtPath_TreatsOfferWorkbookAsCostingCandidate(t *testing.T) {
	path := filepath.Join("245 - BAPCO spare", "OFFER", "245-25 BAPCO SPARE .xlsx")
	if got := classifyDiscoveredFileAtPath(path); got != "costing_sheet" {
		t.Fatalf("expected offer workbook to classify as costing_sheet, got %q", got)
	}

	numberedOfferPath := filepath.Join("101 ENGIE AIT", "OFFER", "REV-2", "101 ENGIE AIT.xlsx")
	if got := classifyDiscoveredFileAtPath(numberedOfferPath); got != "costing_sheet" {
		t.Fatalf("expected numbered offer workbook to classify as costing_sheet, got %q", got)
	}

	siteRequirementPath := filepath.Join("01 EWA-FIT(FEED)", "OFFER", "EWA-WTD-25EMAG -FIT", "Site requirement flowmeter size.xlsx")
	if got := classifyDiscoveredFileAtPath(siteRequirementPath); got == "costing_sheet" {
		t.Fatalf("expected site requirement workbook not to classify as costing_sheet")
	}

	rfqPath := filepath.Join("245 - BAPCO spare", "RFQ", "BOM.xlsx")
	if got := classifyDiscoveredFileAtPath(rfqPath); got == "costing_sheet" {
		t.Fatalf("expected RFQ BOM workbook not to classify as costing_sheet")
	}
}

func TestMergeDiscoveredFiles_DeDuplicatesRecursiveSectionFiles(t *testing.T) {
	sectionFile := DiscoveredFile{FilePath: filepath.Join("142", "RFQ", "request.msg"), FileType: "email"}
	costingFile := DiscoveredFile{FilePath: filepath.Join("142", "REV-1", "PH Trading Costing MasterFile.xlsx"), FileType: "costing_sheet"}

	merged := mergeDiscoveredFiles([]DiscoveredFile{sectionFile}, []DiscoveredFile{sectionFile, costingFile})
	if len(merged) != 2 {
		t.Fatalf("expected 2 de-duplicated files, got %d: %#v", len(merged), merged)
	}
}

func TestMatchCustomerToFolder_UsesLocalCustomerAliases(t *testing.T) {
	customers := []CustomerMaster{
		{Base: Base{ID: "airmech"}, BusinessName: "AIRMECH COMPANY W.L.L"},
		{Base: Base{ID: "mow"}, BusinessName: "Ministry of Works"},
		{Base: Base{ID: "kst"}, BusinessName: "KRAUM AND SPRUK TECHNOLOGIES W.L.L"},
		{Base: Base{ID: "adc"}, BusinessName: "AWAL DAIRY COMPANY W.L.L."},
		{Base: Base{ID: "emaar"}, BusinessName: "EMAAR"},
	}

	cases := map[string]string{
		"150-AIRMENCH- FIT": "airmech",
		"316 MOW LIT":       "mow",
		"310-KST SGR":       "kst",
		"239-ADC FIT":       "adc",
		"99 EMMAR":          "emaar",
	}

	for folderName, wantID := range cases {
		matches := matchCustomerToFolder(folderName, customers)
		if len(matches) == 0 {
			t.Fatalf("expected a match for %q", folderName)
		}
		if matches[0].CustomerID != wantID {
			t.Fatalf("expected %q to match %q, got %q (%s)", folderName, wantID, matches[0].CustomerID, matches[0].BusinessName)
		}
	}
}

func TestIsOneDriveSectionDir_IncludesWorking(t *testing.T) {
	if !isOneDriveSectionDir("WORKING") {
		t.Fatal("expected WORKING to be treated as a OneDrive section directory")
	}
}

func TestCanonicalOneDriveSectionName_NumberedPrefixes(t *testing.T) {
	cases := map[string]string{
		"1_RFQ":                 "RFQ",
		"2_OFFER":               "OFFER",
		"3_FINAL EXECUTION":     "EXECUTION",
		"OFFER _ ASIA":          "OFFER",
		"Offers No 1-50 (2025)": "",
	}

	for name, want := range cases {
		if got := canonicalOneDriveSectionName(name); got != want {
			t.Fatalf("expected %q to canonicalize to %q, got %q", name, want, got)
		}
	}
}

func TestHasStructuredOneDriveDealIdentity_DoesNotTreatBatchRangeAsDeal(t *testing.T) {
	if hasStructuredOneDriveDealIdentity("Offers No 1-50 (2025)") {
		t.Fatal("batch folder name should not be treated as a structured deal")
	}
	if hasStructuredOneDriveDealIdentity("offers 2025") {
		t.Fatal("year container folder should not be treated as a structured deal")
	}
}

func TestIsDirectFileOneDriveDealPath_AllowsBatchContainerChildren(t *testing.T) {
	root := filepath.Clean("/tmp/offers 2025")

	if !isDirectFileOneDriveDealPath(filepath.Join(root, "30 ARLA CALIBRATION"), root) {
		t.Fatal("expected numbered direct child of root to be eligible")
	}

	batchChild := filepath.Join(root, "Offers No 51 -100 (2025)", "100 AHS SP")
	if !isDirectFileOneDriveDealPath(batchChild, root) {
		t.Fatal("expected numbered folder inside batch container to be eligible")
	}

	deepChild := filepath.Join(root, "Offers No 51 -100 (2025)", "100 AHS SP", "archive")
	if isDirectFileOneDriveDealPath(deepChild, root) {
		t.Fatal("expected deeper non-batch descendants to stay ineligible")
	}
}

func TestHasExtractableOneDriveChildDeal_DetectsNestedSectionDeal(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "24 ARLA-FIT")
	if err := os.MkdirAll(filepath.Join(parent, "OTHERS", "OFFER"), 0o755); err != nil {
		t.Fatalf("failed to create nested section deal: %v", err)
	}

	if !hasExtractableOneDriveChildDeal(parent) {
		t.Fatal("expected parent with nested OFFER section to report extractable child deal")
	}

	emptyParent := filepath.Join(root, "190 - ARLA FIT")
	if err := os.MkdirAll(emptyParent, 0o755); err != nil {
		t.Fatalf("failed to create empty parent: %v", err)
	}
	if hasExtractableOneDriveChildDeal(emptyParent) {
		t.Fatal("expected empty parent to have no extractable child deal")
	}
}

func TestParseOneDriveFolderMeta_CleansLooseNumericPrefix(t *testing.T) {
	meta := parseOneDriveFolderMeta("22- Nomac ait")
	if meta.FolderNumber != "22" {
		t.Fatalf("expected folder number 22, got %q", meta.FolderNumber)
	}
	if meta.Title != "Nomac ait" {
		t.Fatalf("expected title Nomac ait, got %q", meta.Title)
	}

	meta = parseOneDriveFolderMeta("150-AIRMENCH- FIT")
	if meta.FolderNumber != "150" {
		t.Fatalf("expected folder number 150, got %q", meta.FolderNumber)
	}
	if meta.Title != "AIRMENCH FIT" {
		t.Fatalf("expected title AIRMENCH FIT, got %q", meta.Title)
	}

	meta = parseOneDriveFolderMeta("307-KST TIT")
	if meta.FolderNumber != "307" {
		t.Fatalf("expected folder number 307, got %q", meta.FolderNumber)
	}
	if meta.Title != "KST TIT" {
		t.Fatalf("expected title KST TIT, got %q", meta.Title)
	}
}

func TestInheritOneDriveParentFolderNumber_ForNestedUnnumberedDeal(t *testing.T) {
	got := inheritOneDriveParentFolderNumber("24 ARLA-FIT", "OTHERS")
	if got != "24 OTHERS" {
		t.Fatalf("expected nested deal name to inherit parent folder number, got %q", got)
	}

	got = inheritOneDriveParentFolderNumber("24 ARLA-FIT", "24 - A")
	if got != "24 - A" {
		t.Fatalf("expected already-numbered nested deal to stay unchanged, got %q", got)
	}
}

func TestExtractDealSectionPaths_IncludesExecutionAliases(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"RFQ", "OFFER", "FINAL EXECUTION", "PROJECT EXECUTION", "SUBMISSION"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("failed to create section %s: %v", name, err)
		}
	}

	sections, err := extractDealSectionPaths(root)
	if err != nil {
		t.Fatalf("extractDealSectionPaths failed: %v", err)
	}

	for _, key := range []string{"RFQ", "OFFER", "EXECUTION", "FINAL EXECUTION", "PROJECT EXECUTION", "SUBMISSION"} {
		if sections[key] == "" {
			t.Fatalf("expected section key %q to be present, got %#v", key, sections)
		}
	}
}
