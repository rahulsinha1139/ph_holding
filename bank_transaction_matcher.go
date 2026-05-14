// =============================================================================
// BANK TRANSACTION MATCHER SERVICE
//
// MISSION: Auto-match bank statement lines to invoices and payments
// FEATURES: Type detection, invoice extraction, customer/supplier matching
// =============================================================================

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

// =============================================================================
// TRANSACTION TYPE CONSTANTS
// =============================================================================

const (
	TxTypeCustomerPayment     = "CUSTOMER_PAYMENT"
	TxTypeChequeReceived      = "CHEQUE_RECEIVED"
	TxTypeSupplierPayment     = "SUPPLIER_PAYMENT"
	TxTypeChequeIssued        = "CHEQUE_ISSUED"
	TxTypeBankFee             = "BANK_FEE"
	TxTypeSwiftCharge         = "SWIFT_CHARGE"
	TxTypeVATOnFee            = "VAT_ON_FEE"
	TxTypeBGFee               = "BG_FEE"
	TxTypeCorrespondentCharge = "CORRESPONDENT_CHARGE"
	TxTypeInternalTransfer    = "INTERNAL_TRANSFER"
	TxTypeFXConversion        = "FX_CONVERSION"
	TxTypeSalary              = "SALARY"
	TxTypeBillPayment         = "BILL_PAYMENT"
	TxTypeOther               = "OTHER"
)

// BankReconciliationMatchResult holds the result of auto-matching
type BankReconciliationMatchResult struct {
	MatchedCount     int     `json:"matched_count"`
	UnmatchedCount   int     `json:"unmatched_count"`
	TotalLines       int     `json:"total_lines"`
	MatchedPercent   float64 `json:"matched_percent"`
	AutoMatchedCount int     `json:"auto_matched_count"`
}

// AllocationInput for split payment allocation
type AllocationInput struct {
	AllocationType  string  `json:"allocation_type"` // CUSTOMER_INVOICE, SUPPLIER_INVOICE, EXPENSE
	EntityID        string  `json:"entity_id"`
	AllocatedAmount float64 `json:"allocated_amount"`
}

// =============================================================================
// AUTO-MATCHING
// =============================================================================

// AutoMatchBankLines attempts to auto-match all unmatched lines in a statement
func (a *App) AutoMatchBankLines(statementID string) (*BankReconciliationMatchResult, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get statement
	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return nil, fmt.Errorf("statement not found: %w", err)
	}

	if statement.Status == "Reconciled" {
		return nil, fmt.Errorf("statement already reconciled")
	}

	// Get unmatched lines
	var lines []BankStatementLine
	a.db.Where("bank_statement_id = ? AND is_matched = ?", statementID, false).Find(&lines)

	// Get known customers and suppliers for matching
	var customerNames []string
	var supplierNames []string
	a.db.Model(&CustomerMaster{}).Pluck("business_name", &customerNames)
	a.db.Model(&SupplierMaster{}).Pluck("supplier_name", &supplierNames)

	matchedCount := 0
	autoMatchedCount := 0
	statementDivision := normalizeDivisionName(statement.Division)
	if strings.TrimSpace(statementDivision) == "" {
		statementDivision = a.resolveBankAccountDivision(statement.BankAccountID)
	}

	for i := range lines {
		line := &lines[i]

		// Step 1: Categorize the transaction
		isCredit := line.Credit > 0
		txType := detectTransactionType(line.Description, isCredit)

		// Step 2: Extract invoice numbers and entity names
		invoiceNumbers := extractInvoiceNumbers(line.Description)
		poNumbers := extractPONumbers(line.Description)
		customerName, supplierName := extractBankEntityName(line.Description, customerNames, supplierNames)

		// Update line with extracted data
		updates := map[string]interface{}{
			"transaction_type":     txType,
			"extracted_invoices":   toJSONArray(invoiceNumbers),
			"extracted_po_numbers": toJSONArray(poNumbers),
			"extracted_customer":   customerName,
			"extracted_supplier":   supplierName,
		}

		// Step 3: Attempt matching based on transaction type
		var matched bool
		var matchedID string
		var confidence float64

		switch txType {
		case TxTypeCustomerPayment, TxTypeChequeReceived:
			matched, matchedID, confidence = a.matchToCustomerInvoice(line, invoiceNumbers, customerName, statementDivision)
		case TxTypeSupplierPayment, TxTypeChequeIssued:
			matched, matchedID, confidence = a.matchToSupplierPayment(line, poNumbers, supplierName, statementDivision)
		case TxTypeBankFee, TxTypeSwiftCharge, TxTypeVATOnFee, TxTypeBGFee, TxTypeCorrespondentCharge:
			// Auto-create expense entry for bank fees
			a.createExpenseFromBankLine(line, txType)
			matched = true
			confidence = 1.0
		}

		if matched {
			updates["is_matched"] = true
			updates["match_type"] = "Auto"
			updates["match_confidence"] = confidence
			updates["matched_invoice_ids"] = matchedID
			matchedCount++
			autoMatchedCount++

			// Log the match
			lineID := line.ID
			a.LogReconciliationAction(statementID, &lineID, "MATCH",
				map[string]interface{}{
					"type":       "Auto",
					"tx_type":    txType,
					"matched_to": matchedID,
					"confidence": confidence,
				},
				"System", true, confidence, "Auto-matched")
		}

		a.db.Model(line).Updates(updates)
	}

	// Count final matched/unmatched
	var totalLines, finalMatchedCount int64
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ?", statementID).Count(&totalLines)
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, true).Count(&finalMatchedCount)

	matchedPercent := 0.0
	if totalLines > 0 {
		matchedPercent = float64(finalMatchedCount) / float64(totalLines) * 100
	}

	// Update statement status
	newStatus := "InProgress"
	if finalMatchedCount == totalLines {
		newStatus = "InProgress" // Still needs verification before reconciled
	}
	a.db.Model(&statement).Update("status", newStatus)

	log.Printf("🔄 Auto-matched %d/%d lines (%.1f%%) for statement %s",
		matchedCount, totalLines, matchedPercent, statementID)

	return &BankReconciliationMatchResult{
		MatchedCount:     int(finalMatchedCount),
		UnmatchedCount:   int(totalLines - finalMatchedCount),
		TotalLines:       int(totalLines),
		MatchedPercent:   matchedPercent,
		AutoMatchedCount: autoMatchedCount,
	}, nil
}

// =============================================================================
// TYPE DETECTION
// =============================================================================

// detectTransactionType determines the transaction type from description
func detectTransactionType(description string, isCredit bool) string {
	desc := strings.ToUpper(description)

	// Credits (money in)
	if isCredit {
		if strings.Contains(desc, "FAWRI") && strings.Contains(desc, "TRANSFER") {
			return TxTypeCustomerPayment
		}
		if strings.Contains(desc, "CHEQUE") || strings.Contains(desc, "CHQ") {
			return TxTypeChequeReceived
		}
		if strings.Contains(desc, "INTERNAL") || strings.Contains(desc, "NBB TRF FROM") {
			return TxTypeInternalTransfer
		}
		return TxTypeCustomerPayment // Default for credits
	}

	// Debits (money out)
	if strings.Contains(desc, "SWIFT FEE") || strings.Contains(desc, "SWIFT CHG") {
		return TxTypeSwiftCharge
	}
	if strings.Contains(desc, "VAT") && (strings.Contains(desc, "FEE") || strings.Contains(desc, "CHARGE")) {
		return TxTypeVATOnFee
	}
	if strings.Contains(desc, "EFTS CHARGE") || strings.Contains(desc, "SERVICE CHARGE") {
		return TxTypeBankFee
	}
	if strings.Contains(desc, "AMENDBG") || strings.Contains(desc, "BGG") || strings.Contains(desc, "GUARANTEE") {
		return TxTypeBGFee
	}
	if strings.Contains(desc, "CORR BNK CHG") || strings.Contains(desc, "CORRESPONDENT") {
		return TxTypeCorrespondentCharge
	}
	if strings.Contains(desc, "NBB TRF FROM") && strings.Contains(desc, "TO") {
		return TxTypeInternalTransfer
	}
	if strings.Contains(desc, "FAWATEER") || strings.Contains(desc, "NBL-") {
		return TxTypeBillPayment
	}
	if strings.Contains(desc, "CHQ NO") || strings.Contains(desc, "CHEQUE") {
		return TxTypeChequeIssued
	}
	if strings.Contains(desc, "DGC") || strings.Contains(desc, "FAWRI") || strings.Contains(desc, "REMITTANCE") {
		return TxTypeSupplierPayment
	}
	if strings.Contains(desc, "SALARY") || strings.Contains(desc, "PAYROLL") {
		return TxTypeSalary
	}

	return TxTypeOther
}

// =============================================================================
// EXTRACTION
// =============================================================================

// extractInvoiceNumbers extracts invoice numbers from description
func extractInvoiceNumbers(description string) []string {
	var invoices []string

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)/INV/([A-Z0-9/-]+)`),           // /INV/PH25/091
		regexp.MustCompile(`(?i)PH(\d{2})[/-]?(\d{3,4})`),      // PH25/091, PH25-091, PH24159
		regexp.MustCompile(`(?i)INV[/-]?(\d{4})[/-](\d{3,4})`), // INV-2026-001
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(description, -1)
		invoices = append(invoices, matches...)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	unique := []string{}
	for _, inv := range invoices {
		if !seen[inv] {
			seen[inv] = true
			unique = append(unique, inv)
		}
	}

	return unique
}

// extractPONumbers extracts purchase order numbers from description
func extractPONumbers(description string) []string {
	var pos []string

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)PO[/-]?(\d{4})[/-](\d{3,4})`), // PO-2025-001
		regexp.MustCompile(`(?i)PUR[/-]?(\d+)`),               // PUR-12345
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(description, -1)
		pos = append(pos, matches...)
	}

	return pos
}

// extractBankEntityName extracts customer or supplier name from description
func extractBankEntityName(description string, knownCustomers, knownSuppliers []string) (customer, supplier string) {
	desc := strings.ToUpper(description)

	// Check against known customers
	for _, cust := range knownCustomers {
		if strings.Contains(desc, strings.ToUpper(cust)) {
			return cust, ""
		}
	}

	// Check against known suppliers
	for _, supp := range knownSuppliers {
		if strings.Contains(desc, strings.ToUpper(supp)) {
			return "", supp
		}
	}

	// Try to extract from common patterns
	if strings.Contains(desc, "TRANSFER") {
		parts := strings.Split(desc, "TRANSFER")
		if len(parts) > 1 {
			name := strings.TrimSpace(parts[1])
			name = strings.Split(name, " FTRF")[0]
			name = strings.Split(name, " W.L.L")[0]
			name = strings.Split(name, " WLL")[0]
			name = strings.Split(name, " B.S.C")[0]
			if len(name) > 3 {
				return strings.TrimSpace(name), ""
			}
		}
	}

	return "", ""
}

// =============================================================================
// INVOICE MATCHING
// =============================================================================

// matchToCustomerInvoice attempts to match a credit line to customer invoices
func (a *App) matchToCustomerInvoice(line *BankStatementLine, invoiceNumbers []string, customerName string, divisions ...string) (bool, string, float64) {
	division := "PH Trading"
	if len(divisions) > 0 {
		division = divisions[0]
	}
	division = normalizeDivisionName(division)
	if line.Credit <= 0 {
		return false, "", 0
	}
	openStatuses := []string{"Sent", "Overdue", "PartiallyPaid", "Paid"}

	// Try matching by invoice number first
	for _, invNum := range invoiceNumbers {
		var invoice Invoice
		err := a.db.Where("invoice_number LIKE ? ESCAPE '\\\\' AND division = ? AND status IN ?", "%"+escapeLikeWildcards(invNum)+"%", division, openStatuses).First(&invoice).Error
		if err == nil {
			payable := payableCustomerInvoiceAmount(invoice)
			if line.Credit <= payable+bankMatchTolerance(payable) || bankAmountsClose(line.Credit, payable) {
				return true, invoice.ID, 0.95
			}
		}
	}

	// Try matching by customer name and amount
	if customerName != "" {
		var customer CustomerMaster
		err := a.db.Where("business_name LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(customerName)+"%").First(&customer).Error
		if err == nil {
			// Found customer, look for matching invoice amount
			var invoices []Invoice
			err = a.db.Where("customer_id = ? AND division = ? AND status IN ?",
				customer.ID, division, openStatuses).Find(&invoices).Error
			if err == nil {
				for _, invoice := range invoices {
					if bankAmountsClose(line.Credit, payableCustomerInvoiceAmount(invoice)) {
						return true, invoice.ID, 0.85
					}
				}
			}
		}
	}

	// Try matching by amount only (low confidence)
	var invoices []Invoice
	err := a.db.Where("division = ? AND status IN ?", division, openStatuses).Limit(100).Find(&invoices).Error
	if err == nil {
		for _, invoice := range invoices {
			if bankAmountsClose(line.Credit, payableCustomerInvoiceAmount(invoice)) {
				return true, invoice.ID, 0.60
			}
		}
	}

	return false, "", 0
}

// matchToSupplierPayment attempts to match a debit line to supplier payments
func (a *App) matchToSupplierPayment(line *BankStatementLine, poNumbers []string, supplierName string, division string) (bool, string, float64) {
	division = normalizeDivisionName(division)
	if line.Debit <= 0 {
		return false, "", 0
	}
	// Try matching by PO number
	for _, poNum := range poNumbers {
		var invoice SupplierInvoice
		err := a.db.Where("po_number LIKE ? ESCAPE '\\\\' AND division = ? AND status NOT IN ?",
			"%"+escapeLikeWildcards(poNum)+"%", division, []string{"Paid", "Rejected", "Cancelled", "Void"}).First(&invoice).Error
		if err == nil {
			if bankAmountsClose(line.Debit, invoice.TotalBHD) {
				return true, invoice.ID, 0.90
			}
		}
	}

	// Try matching by supplier name and amount
	if supplierName != "" {
		var supplier SupplierMaster
		err := a.db.Where("supplier_name LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(supplierName)+"%").First(&supplier).Error
		if err == nil {
			var invoice SupplierInvoice
			err = a.db.Where("supplier_id = ? AND total_bhd = ? AND division = ? AND status != ?",
				supplier.ID, line.Debit, division, "Paid").First(&invoice).Error
			if err == nil {
				return true, invoice.ID, 0.80
			}
		}
	}

	return false, "", 0
}

// =============================================================================
// EXPENSE CREATION
// =============================================================================

// createExpenseFromBankLine creates an expense entry for bank fees
func (a *App) createExpenseFromBankLine(line *BankStatementLine, txType string) error {
	expense := BankExpenseEntry{
		BankStatementLineID: line.ID,
		Division:            a.resolveBankStatementDivision(line.BankStatementID),
		ExpenseDate:         line.TransactionDate,
		Description:         line.Description,
		Category:            txType,
		Amount:              line.Debit,
		Currency:            "BHD",
	}

	// Add VAT if this is a VAT entry
	if txType == TxTypeVATOnFee {
		expense.VATAmount = line.Debit
	}

	return a.db.Create(&expense).Error
}

// =============================================================================
// MANUAL MATCHING
// =============================================================================

// ManualMatchLine manually matches a bank line to an entity
func (a *App) ManualMatchLine(lineID, entityType, entityID, user string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	line, statement, err := a.getBankStatementForLine(lineID)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "match transactions"); err != nil {
		return err
	}
	statementDivision := a.resolveBankStatementDivision(line.BankStatementID)

	if line.IsMatched {
		return fmt.Errorf("line already matched")
	}

	// Validate amount ratio between bank line and matched entity
	var entityAmount float64
	lineAmt := line.Credit
	if lineAmt == 0 {
		lineAmt = line.Debit
	}
	switch entityType {
	case "CUSTOMER_INVOICE":
		if line.Credit <= 0 {
			return fmt.Errorf("customer invoices must be matched to credit transactions")
		}
		var inv Invoice
		if err := a.db.First(&inv, "id = ?", entityID).Error; err == nil {
			if normalizeDivisionName(inv.Division) != statementDivision {
				return fmt.Errorf("cannot match %s bank line to %s invoice", statementDivision, normalizeDivisionName(inv.Division))
			}
			entityAmount = payableCustomerInvoiceAmount(inv)
		} else {
			return fmt.Errorf("customer invoice not found: %w", err)
		}
	case "CUSTOMER_PAYMENT":
		if line.Credit <= 0 {
			return fmt.Errorf("customer payments must be matched to credit transactions")
		}
		var payment Payment
		if err := a.db.First(&payment, "id = ?", entityID).Error; err == nil {
			if normalizeDivisionName(payment.Division) != statementDivision {
				return fmt.Errorf("cannot match %s bank line to %s customer payment", statementDivision, normalizeDivisionName(payment.Division))
			}
			entityAmount = payment.AmountBHD
		} else {
			return fmt.Errorf("customer payment not found: %w", err)
		}
	case "CUSTOMER_RECEIPT":
		if line.Credit <= 0 {
			return fmt.Errorf("customer receipts must be matched to credit transactions")
		}
		var receipt CustomerReceipt
		if err := a.db.First(&receipt, "id = ?", entityID).Error; err == nil {
			if normalizeDivisionName(receipt.Division) != statementDivision {
				return fmt.Errorf("cannot match %s bank line to %s customer receipt", statementDivision, normalizeDivisionName(receipt.Division))
			}
			if receipt.Status == "Reversed" {
				return fmt.Errorf("reversed receipts cannot be reconciled")
			}
			entityAmount = receipt.AmountBHD
		} else {
			return fmt.Errorf("customer receipt not found: %w", err)
		}
	case "SUPPLIER_INVOICE":
		if line.Debit <= 0 {
			return fmt.Errorf("supplier invoices must be matched to debit transactions")
		}
		var sinv SupplierInvoice
		if err := a.db.First(&sinv, "id = ?", entityID).Error; err == nil {
			if a.resolveSupplierInvoiceDivision(sinv) != statementDivision {
				return fmt.Errorf("cannot match %s bank line to %s supplier invoice", statementDivision, a.resolveSupplierInvoiceDivision(sinv))
			}
			entityAmount = sinv.TotalBHD
		} else {
			return fmt.Errorf("supplier invoice not found: %w", err)
		}
	case "SUPPLIER_PAYMENT":
		if line.Debit <= 0 {
			return fmt.Errorf("supplier payments must be matched to debit transactions")
		}
		var payment SupplierPayment
		if err := a.db.First(&payment, "id = ?", entityID).Error; err == nil {
			if normalizeDivisionName(payment.Division) != statementDivision {
				return fmt.Errorf("cannot match %s bank line to %s supplier payment", statementDivision, normalizeDivisionName(payment.Division))
			}
			entityAmount = payment.AmountBHD
		} else {
			return fmt.Errorf("supplier payment not found: %w", err)
		}
	case "PAYROLL_PAYOUT":
		if line.Debit <= 0 {
			return fmt.Errorf("payroll payouts must be matched to debit transactions")
		}
		var payout PayrollPayout
		if err := a.db.First(&payout, "id = ?", entityID).Error; err == nil {
			entityAmount = payout.Amount
		} else {
			return fmt.Errorf("payroll payout not found: %w", err)
		}
	case "EXPENSE":
		if line.Debit <= 0 {
			return fmt.Errorf("expenses must be matched to debit transactions")
		}
		var expense ExpenseEntry
		if err := a.db.First(&expense, "id = ?", entityID).Error; err == nil {
			if normalizeDivisionName(expense.Division) != statementDivision {
				return fmt.Errorf("cannot match %s bank line to %s expense", statementDivision, normalizeDivisionName(expense.Division))
			}
			if !isExpenseReconcilable(expense) {
				return fmt.Errorf("expense %s is not open for bank reconciliation", firstNonEmptyString(expense.EntryNumber, expense.ID))
			}
			entityAmount = payableExpenseEntryAmount(expense)
		} else {
			return fmt.Errorf("expense not found: %w", err)
		}
	default:
		return fmt.Errorf("unsupported match type: %s", entityType)
	}
	// Reject zero-amount entities — a zero-value invoice skips all ratio checks,
	// allowing any bank line to reconcile against a phantom or draft invoice.
	if entityAmount <= 0 {
		return fmt.Errorf("cannot match: entity amount is %.3f BHD — only positive-value invoices can be reconciled", entityAmount)
	}
	if lineAmt > 0 {
		tolerance := bankMatchTolerance(entityAmount)
		if entityType == "CUSTOMER_INVOICE" {
			if lineAmt > entityAmount+tolerance {
				return fmt.Errorf("amount mismatch: bank receipt %.3f BHD exceeds invoice outstanding %.3f BHD by more than %.3f BHD", lineAmt, entityAmount, tolerance)
			}
		} else if !bankAmountsClose(lineAmt, entityAmount) {
			ratio := lineAmt / entityAmount
			return fmt.Errorf("amount mismatch: bank line %.3f BHD vs entity %.3f BHD (%.1f%% difference, tolerance ±5%%)", lineAmt, entityAmount, (ratio-1)*100)
		}
	}

	updates := map[string]interface{}{
		"is_matched":       true,
		"match_type":       "Manual",
		"match_confidence": 1.0,
	}

	switch entityType {
	case "CUSTOMER_INVOICE":
		updates["matched_invoice_ids"] = entityID
	case "CUSTOMER_PAYMENT", "CUSTOMER_RECEIPT":
		updates["matched_payment_id"] = entityID
	case "SUPPLIER_INVOICE":
		updates["matched_invoice_ids"] = entityID
	case "SUPPLIER_PAYMENT":
		updates["matched_payment_id"] = entityID
	case "EXPENSE":
		updates["matched_expense_id"] = entityID
	case "PAYROLL_PAYOUT":
		var payout PayrollPayout
		if err := a.db.First(&payout, "id = ?", entityID).Error; err != nil {
			return fmt.Errorf("payroll payout not found: %w", err)
		}
		var run PayrollRun
		if err := a.db.First(&run, "id = ?", payout.PayrollRunID).Error; err == nil && run.PayoutJournalEntryID != nil {
			updates["matched_journal_id"] = *run.PayoutJournalEntryID
		}
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if entityType == "PAYROLL_PAYOUT" {
			var payout PayrollPayout
			if err := tx.First(&payout, "id = ?", entityID).Error; err != nil {
				return fmt.Errorf("payroll payout not found: %w", err)
			}
			lineIDRef := line.ID
			if err := tx.Model(&payout).Updates(map[string]interface{}{
				"bank_statement_line_id": &lineIDRef,
				"updated_at":             time.Now(),
			}).Error; err != nil {
				return fmt.Errorf("failed to link payroll payout to bank line: %w", err)
			}
		}

		if err := tx.Model(line).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update line: %w", err)
		}

		return a.logReconciliationActionWithDB(
			tx,
			line.BankStatementID,
			&lineID,
			"MATCH",
			map[string]interface{}{
				"type":        "Manual",
				"entity_type": entityType,
				"entity_id":   entityID,
			},
			user,
			false,
			1.0,
			"Manual match",
		)
	}); err != nil {
		return err
	}

	log.Printf("🔗 Manual match: Line %s → %s %s", lineID, entityType, entityID)
	return nil
}

func payableCustomerInvoiceAmount(invoice Invoice) float64 {
	if invoice.OutstandingBHD > 0 {
		return invoice.OutstandingBHD
	}
	if strings.EqualFold(strings.TrimSpace(invoice.Status), "Paid") {
		return 0
	}
	return invoice.GrandTotalBHD
}

func payableExpenseEntryAmount(expense ExpenseEntry) float64 {
	if expense.TotalAmount > 0 {
		return expense.TotalAmount
	}
	if expense.Amount+expense.VATAmount > 0 {
		return expense.Amount + expense.VATAmount
	}
	return expense.Amount
}

func isExpenseReconcilable(expense ExpenseEntry) bool {
	status := strings.ToLower(strings.TrimSpace(expense.Status))
	paymentStatus := strings.ToLower(strings.TrimSpace(expense.PaymentStatus))
	if paymentStatus == "paid" {
		return false
	}
	switch status {
	case "rejected", "cancelled", "canceled", "void":
		return false
	default:
		return true
	}
}

func bankMatchTolerance(amount float64) float64 {
	return math.Max(0.001, math.Abs(amount)*0.05)
}

func bankAmountsClose(a, b float64) bool {
	return math.Abs(a-b) <= bankMatchTolerance(math.Max(math.Abs(a), math.Abs(b)))
}

// UnmatchLine removes a match from a bank line
func (a *App) UnmatchLine(lineID, user, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var line BankStatementLine
	if err := a.db.First(&line, "id = ?", lineID).Error; err != nil {
		return fmt.Errorf("line not found: %w", err)
	}

	previousMatch := line.MatchedInvoiceIDs
	if previousMatch == "" {
		previousMatch = line.MatchedPaymentID
	}

	updates := map[string]interface{}{
		"is_matched":          false,
		"match_type":          "Unmatched",
		"match_confidence":    0,
		"matched_invoice_ids": "",
		"matched_payment_id":  "",
		"matched_expense_id":  nil,
		"matched_journal_id":  "",
	}

	if err := a.db.Where("bank_statement_line_id = ?", lineID).Delete(&BankLinePaymentAllocation{}).Error; err != nil {
		return fmt.Errorf("failed to clear split allocations: %w", err)
	}

	var payout PayrollPayout
	if err := a.db.Where("bank_statement_line_id = ?", lineID).First(&payout).Error; err == nil {
		if updateErr := a.db.Model(&payout).Updates(map[string]interface{}{
			"bank_statement_line_id": nil,
			"updated_at":             time.Now(),
		}).Error; updateErr != nil {
			return fmt.Errorf("failed to unlink payroll payout from bank line: %w", updateErr)
		}
	}

	if err := a.db.Model(&line).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to unmatch line: %w", err)
	}

	// Log the action
	a.LogReconciliationAction(line.BankStatementID, &lineID, "UNMATCH",
		map[string]interface{}{
			"previous_match": previousMatch,
			"reason":         reason,
		},
		user, false, 1.0, reason)

	log.Printf("🔓 Unmatched: Line %s by %s: %s", lineID, user, reason)
	return nil
}

// =============================================================================
// SPLIT ALLOCATION
// =============================================================================

// CreateSplitAllocation splits a bank line across multiple entities
func (a *App) CreateSplitAllocation(lineID string, allocations []AllocationInput, user string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if len(allocations) == 0 {
		return fmt.Errorf("at least one allocation is required")
	}

	var line BankStatementLine
	var allocationIDs []string

	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&line, "id = ?", lineID).Error; err != nil {
			return fmt.Errorf("line not found: %w", err)
		}
		if line.IsMatched {
			return fmt.Errorf("line already matched")
		}

		var statement BankStatement
		if err := tx.First(&statement, "id = ?", line.BankStatementID).Error; err != nil {
			return fmt.Errorf("statement not found: %w", err)
		}
		statementDivision := normalizeDivisionName(statement.Division)
		if statementDivision == "" {
			var account CompanyBankAccount
			if err := tx.First(&account, "id = ?", statement.BankAccountID).Error; err == nil {
				statementDivision = normalizeDivisionName(account.Division)
			}
		}
		if statementDivision == "" {
			statementDivision = "PH Trading"
		}

		lineAmount := line.Credit
		if lineAmount == 0 {
			lineAmount = line.Debit
		}
		if lineAmount <= 0 {
			return fmt.Errorf("line amount must be positive")
		}
		const bhdRoundingTolerance = 0.001

		var totalAllocated float64
		seen := make(map[string]bool)
		allocationRecords := make([]BankLinePaymentAllocation, 0, len(allocations))
		allocationIDs = make([]string, 0, len(allocations))

		for _, alloc := range allocations {
			allocationType := strings.ToUpper(strings.TrimSpace(alloc.AllocationType))
			entityID := strings.TrimSpace(alloc.EntityID)
			allocatedAmount := roundBHD(alloc.AllocatedAmount)
			if entityID == "" {
				return fmt.Errorf("allocation target is required")
			}
			if allocatedAmount <= 0 {
				return fmt.Errorf("allocation amount for %s must be positive", entityID)
			}
			key := allocationType + ":" + entityID
			if seen[key] {
				return fmt.Errorf("duplicate allocation target: %s", entityID)
			}
			seen[key] = true

			allocation := BankLinePaymentAllocation{
				BankStatementLineID: lineID,
				AllocationType:      allocationType,
				AllocatedAmount:     allocatedAmount,
				Currency:            "BHD",
				Status:              "Allocated",
			}

			switch allocationType {
			case "CUSTOMER_INVOICE":
				if line.Credit <= 0 {
					return fmt.Errorf("customer invoices must be allocated to credit transactions")
				}
				var invoice Invoice
				if err := tx.First(&invoice, "id = ?", entityID).Error; err != nil {
					return fmt.Errorf("customer invoice not found: %w", err)
				}
				if normalizeDivisionName(invoice.Division) != statementDivision {
					return fmt.Errorf("cannot allocate %s bank line to %s invoice", statementDivision, normalizeDivisionName(invoice.Division))
				}
				payable := payableCustomerInvoiceAmount(invoice)
				var previouslyAllocated float64
				if err := tx.Model(&BankLinePaymentAllocation{}).
					Where("customer_invoice_id = ? AND status <> ? AND bank_statement_line_id <> ?", entityID, "Disputed", lineID).
					Select("COALESCE(SUM(allocated_amount), 0)").
					Scan(&previouslyAllocated).Error; err != nil {
					return fmt.Errorf("failed to check existing customer allocations: %w", err)
				}
				remaining := roundBHD(payable - previouslyAllocated)
				if allocatedAmount > remaining+bhdRoundingTolerance {
					return fmt.Errorf("allocation %.3f BHD exceeds remaining invoice balance %.3f BHD", allocatedAmount, remaining)
				}
				idCopy := entityID
				allocation.CustomerInvoiceID = &idCopy
			case "CUSTOMER_PAYMENT":
				if line.Credit <= 0 {
					return fmt.Errorf("customer payments must be allocated to credit transactions")
				}
				var payment Payment
				if err := tx.First(&payment, "id = ?", entityID).Error; err != nil {
					return fmt.Errorf("customer payment not found: %w", err)
				}
				if normalizeDivisionName(payment.Division) != statementDivision {
					return fmt.Errorf("cannot allocate %s bank line to %s customer payment", statementDivision, normalizeDivisionName(payment.Division))
				}
				var previouslyAllocated float64
				if err := tx.Model(&BankLinePaymentAllocation{}).
					Where("customer_payment_id = ? AND status <> ? AND bank_statement_line_id <> ?", entityID, "Disputed", lineID).
					Select("COALESCE(SUM(allocated_amount), 0)").
					Scan(&previouslyAllocated).Error; err != nil {
					return fmt.Errorf("failed to check existing customer payment allocations: %w", err)
				}
				remaining := roundBHD(payment.AmountBHD - previouslyAllocated)
				if allocatedAmount > remaining+bhdRoundingTolerance {
					return fmt.Errorf("allocation %.3f BHD exceeds remaining customer payment amount %.3f BHD", allocatedAmount, remaining)
				}
				idCopy := entityID
				allocation.CustomerPaymentID = &idCopy
			case "CUSTOMER_RECEIPT":
				if line.Credit <= 0 {
					return fmt.Errorf("customer receipts must be allocated to credit transactions")
				}
				var receipt CustomerReceipt
				if err := tx.First(&receipt, "id = ?", entityID).Error; err != nil {
					return fmt.Errorf("customer receipt not found: %w", err)
				}
				if normalizeDivisionName(receipt.Division) != statementDivision {
					return fmt.Errorf("cannot allocate %s bank line to %s customer receipt", statementDivision, normalizeDivisionName(receipt.Division))
				}
				if receipt.Status == "Reversed" {
					return fmt.Errorf("reversed receipts cannot be allocated")
				}
				var previouslyAllocated float64
				if err := tx.Model(&BankLinePaymentAllocation{}).
					Where("customer_receipt_id = ? AND status <> ? AND bank_statement_line_id <> ?", entityID, "Disputed", lineID).
					Select("COALESCE(SUM(allocated_amount), 0)").
					Scan(&previouslyAllocated).Error; err != nil {
					return fmt.Errorf("failed to check existing customer receipt allocations: %w", err)
				}
				remaining := roundBHD(receipt.AmountBHD - previouslyAllocated)
				if allocatedAmount > remaining+bhdRoundingTolerance {
					return fmt.Errorf("allocation %.3f BHD exceeds remaining customer receipt amount %.3f BHD", allocatedAmount, remaining)
				}
				idCopy := entityID
				allocation.CustomerReceiptID = &idCopy
			case "SUPPLIER_INVOICE":
				if line.Debit <= 0 {
					return fmt.Errorf("supplier invoices must be allocated to debit transactions")
				}
				var invoice SupplierInvoice
				if err := tx.First(&invoice, "id = ?", entityID).Error; err != nil {
					return fmt.Errorf("supplier invoice not found: %w", err)
				}
				if a.resolveSupplierInvoiceDivision(invoice) != statementDivision {
					return fmt.Errorf("cannot allocate %s bank line to %s supplier invoice", statementDivision, a.resolveSupplierInvoiceDivision(invoice))
				}
				var previouslyAllocated float64
				if err := tx.Model(&BankLinePaymentAllocation{}).
					Where("supplier_invoice_id = ? AND status <> ? AND bank_statement_line_id <> ?", entityID, "Disputed", lineID).
					Select("COALESCE(SUM(allocated_amount), 0)").
					Scan(&previouslyAllocated).Error; err != nil {
					return fmt.Errorf("failed to check existing supplier allocations: %w", err)
				}
				remaining := roundBHD(invoice.TotalBHD - previouslyAllocated)
				if allocatedAmount > remaining+bhdRoundingTolerance {
					return fmt.Errorf("allocation %.3f BHD exceeds remaining supplier invoice balance %.3f BHD", allocatedAmount, remaining)
				}
				idCopy := entityID
				allocation.SupplierInvoiceID = &idCopy
			case "EXPENSE":
				if line.Debit <= 0 {
					return fmt.Errorf("expenses must be allocated to debit transactions")
				}
				var expense ExpenseEntry
				if err := tx.First(&expense, "id = ?", entityID).Error; err != nil {
					return fmt.Errorf("expense not found: %w", err)
				}
				if normalizeDivisionName(expense.Division) != statementDivision {
					return fmt.Errorf("cannot allocate %s bank line to %s expense", statementDivision, normalizeDivisionName(expense.Division))
				}
				if !isExpenseReconcilable(expense) {
					return fmt.Errorf("expense %s is not open for bank reconciliation", firstNonEmptyString(expense.EntryNumber, expense.ID))
				}
				var previouslyAllocated float64
				if err := tx.Model(&BankLinePaymentAllocation{}).
					Where("expense_entry_id = ? AND status <> ? AND bank_statement_line_id <> ?", entityID, "Disputed", lineID).
					Select("COALESCE(SUM(allocated_amount), 0)").
					Scan(&previouslyAllocated).Error; err != nil {
					return fmt.Errorf("failed to check existing expense allocations: %w", err)
				}
				remaining := roundBHD(payableExpenseEntryAmount(expense) - previouslyAllocated)
				if allocatedAmount > remaining+bhdRoundingTolerance {
					return fmt.Errorf("allocation %.3f BHD exceeds remaining expense balance %.3f BHD", allocatedAmount, remaining)
				}
				idCopy := entityID
				allocation.ExpenseEntryID = &idCopy
			default:
				return fmt.Errorf("unsupported allocation type: %s", allocationType)
			}

			totalAllocated = roundBHD(totalAllocated + allocatedAmount)
			allocationRecords = append(allocationRecords, allocation)
			allocationIDs = append(allocationIDs, entityID)
		}

		diff := math.Abs(totalAllocated - lineAmount)
		if diff > bhdRoundingTolerance {
			return fmt.Errorf("allocation total (%.3f) does not match line amount (%.3f) — difference %.3f exceeds 1 fils tolerance",
				totalAllocated, lineAmount, diff)
		}

		if err := tx.Create(&allocationRecords).Error; err != nil {
			return fmt.Errorf("failed to create allocations: %w", err)
		}

		return tx.Model(&line).Updates(map[string]interface{}{
			"is_matched":          true,
			"match_type":          "Split",
			"match_confidence":    1.0,
			"matched_invoice_ids": toJSONArray(allocationIDs),
		}).Error
	})
	if err != nil {
		return err
	}

	// Log the action
	allocJSON, _ := json.Marshal(allocations)
	a.LogReconciliationAction(line.BankStatementID, &lineID, "SPLIT",
		map[string]interface{}{"allocations": string(allocJSON)},
		user, false, 1.0, "Split allocation")

	log.Printf("✂️ Split allocation created for line %s: %d allocations", lineID, len(allocations))
	return nil
}

// =============================================================================
// BULK CATEGORIZATION
// =============================================================================

// CategorizeTransactions categorizes all lines in a statement
func (a *App) CategorizeTransactions(statementID string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var lines []BankStatementLine
	a.db.Where("bank_statement_id = ?", statementID).Find(&lines)

	var customerNames []string
	var supplierNames []string
	a.db.Model(&CustomerMaster{}).Pluck("business_name", &customerNames)
	a.db.Model(&SupplierMaster{}).Pluck("supplier_name", &supplierNames)

	categorizedCount := 0
	for _, line := range lines {
		isCredit := line.Credit > 0
		txType := detectTransactionType(line.Description, isCredit)
		invoices := extractInvoiceNumbers(line.Description)
		pos := extractPONumbers(line.Description)
		customer, supplier := extractBankEntityName(line.Description, customerNames, supplierNames)

		a.db.Model(&line).Updates(map[string]interface{}{
			"transaction_type":     txType,
			"extracted_invoices":   toJSONArray(invoices),
			"extracted_po_numbers": toJSONArray(pos),
			"extracted_customer":   customer,
			"extracted_supplier":   supplier,
		})
		categorizedCount++
	}

	log.Printf("🏷️ Categorized %d transactions for statement %s", categorizedCount, statementID)
	return nil
}
