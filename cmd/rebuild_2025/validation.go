package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ValidationResult holds the results of a single validation check
type ValidationResult struct {
	Category    string
	CheckName   string
	TotalCount  int64
	ValidCount  int64
	InvalidCount int64
	Percentage  float64
	Status      string // "✅", "⚠️", "❌"
	Details     string
}

// ValidationReport aggregates all validation results
type ValidationReport struct {
	Timestamp        time.Time
	FKChecks         []ValidationResult
	DataQualityChecks []ValidationResult
	StatusChecks     []ValidationResult
	Totals           map[string]interface{}
	OverallStatus    string
}

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

// runValidation runs FK integrity checks and generates a comprehensive validation report
func runValidation(db *gorm.DB) error {
	fmt.Println("\n" + colorYellow + "🔍 RUNNING VALIDATION CHECKS..." + colorReset)

	report := &ValidationReport{
		Timestamp: time.Now(),
		Totals:    make(map[string]interface{}),
	}

	// Run all validation categories
	if err := validateFKIntegrity(db, report); err != nil {
		return fmt.Errorf("FK integrity checks failed: %w", err)
	}

	if err := validateDataQuality(db, report); err != nil {
		return fmt.Errorf("data quality checks failed: %w", err)
	}

	if err := validateStatuses(db, report); err != nil {
		return fmt.Errorf("status validation failed: %w", err)
	}

	if err := verifyTotals(db, report); err != nil {
		return fmt.Errorf("totals verification failed: %w", err)
	}

	// Print comprehensive report
	printValidationReport(report)

	// Determine overall status
	hasErrors := false
	for _, check := range append(append(report.FKChecks, report.DataQualityChecks...), report.StatusChecks...) {
		if check.Status == "❌" {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed - see report above for details")
	}

	fmt.Println("\n" + colorGreen + "✅ ALL VALIDATION CHECKS PASSED" + colorReset)
	return nil
}

// validateFKIntegrity checks foreign key relationships
func validateFKIntegrity(db *gorm.DB, report *ValidationReport) error {
	fmt.Println("\n📋 Checking FK Integrity...")

	// 1. Invoices → Customers
	var totalInvoices, orphanedInvoices int64
	db.Model(&Invoice{}).Count(&totalInvoices)
	db.Raw(`SELECT COUNT(*) FROM invoices
	        WHERE customer_id NOT IN (SELECT id FROM customers)
	        OR customer_id IS NULL OR customer_id = ''`).Scan(&orphanedInvoices)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Invoices → Customers",
		totalInvoices,
		totalInvoices - orphanedInvoices,
		orphanedInvoices,
	))

	// 2. Payments → Invoices
	var totalPayments, orphanedPayments int64
	db.Model(&Payment{}).Count(&totalPayments)
	db.Raw(`SELECT COUNT(*) FROM payments
	        WHERE invoice_id NOT IN (SELECT id FROM invoices)
	        OR invoice_id IS NULL OR invoice_id = ''`).Scan(&orphanedPayments)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Payments → Invoices",
		totalPayments,
		totalPayments - orphanedPayments,
		orphanedPayments,
	))

	// 3. Supplier Invoices → Suppliers
	var totalSupplierInvoices, orphanedSupplierInvoices int64
	db.Model(&SupplierInvoice{}).Count(&totalSupplierInvoices)
	db.Raw(`SELECT COUNT(*) FROM supplier_invoices
	        WHERE supplier_id NOT IN (SELECT id FROM suppliers)
	        OR supplier_id IS NULL OR supplier_id = ''`).Scan(&orphanedSupplierInvoices)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Supplier Invoices → Suppliers",
		totalSupplierInvoices,
		totalSupplierInvoices - orphanedSupplierInvoices,
		orphanedSupplierInvoices,
	))

	// 4. Opportunities → Customers
	var totalOpportunities, orphanedOpportunities int64
	db.Model(&Opportunity{}).Count(&totalOpportunities)
	db.Raw(`SELECT COUNT(*) FROM opportunities
	        WHERE customer_id NOT IN (SELECT id FROM customers)
	        OR customer_id IS NULL OR customer_id = ''`).Scan(&orphanedOpportunities)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Opportunities → Customers",
		totalOpportunities,
		totalOpportunities - orphanedOpportunities,
		orphanedOpportunities,
	))

	// 5. Purchase Orders → Suppliers
	var totalPOs, orphanedPOs int64
	db.Model(&PurchaseOrder{}).Count(&totalPOs)
	db.Raw(`SELECT COUNT(*) FROM purchase_orders
	        WHERE supplier_id NOT IN (SELECT id FROM suppliers)
	        OR supplier_id IS NULL OR supplier_id = ''`).Scan(&orphanedPOs)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Purchase Orders → Suppliers",
		totalPOs,
		totalPOs - orphanedPOs,
		orphanedPOs,
	))

	// 6. Orders → Customers
	var totalOrders, orphanedOrders int64
	db.Model(&Order{}).Count(&totalOrders)
	db.Raw(`SELECT COUNT(*) FROM orders
	        WHERE customer_id NOT IN (SELECT id FROM customers)
	        OR customer_id IS NULL OR customer_id = ''`).Scan(&orphanedOrders)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Orders → Customers",
		totalOrders,
		totalOrders - orphanedOrders,
		orphanedOrders,
	))

	// 7. Delivery Notes → Orders
	var totalDNs, orphanedDNs int64
	db.Model(&DeliveryNote{}).Count(&totalDNs)
	db.Raw(`SELECT COUNT(*) FROM delivery_notes
	        WHERE order_id NOT IN (SELECT id FROM orders)
	        OR order_id IS NULL OR order_id = ''`).Scan(&orphanedDNs)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"Delivery Notes → Orders",
		totalDNs,
		totalDNs - orphanedDNs,
		orphanedDNs,
	))

	// 8. GRNs → Purchase Orders
	var totalGRNs, orphanedGRNs int64
	db.Model(&GoodsReceivedNote{}).Count(&totalGRNs)
	db.Raw(`SELECT COUNT(*) FROM goods_received_notes
	        WHERE purchase_order_id NOT IN (SELECT id FROM purchase_orders)
	        OR purchase_order_id IS NULL OR purchase_order_id = ''`).Scan(&orphanedGRNs)

	report.FKChecks = append(report.FKChecks, createValidationResult(
		"FK Integrity",
		"GRNs → Purchase Orders",
		totalGRNs,
		totalGRNs - orphanedGRNs,
		orphanedGRNs,
	))

	return nil
}

// validateDataQuality checks data completeness and quality
func validateDataQuality(db *gorm.DB, report *ValidationReport) error {
	fmt.Println("\n🔍 Checking Data Quality...")

	// 1. Invoices with NULL amounts
	var totalInvoices, zeroAmountInvoices int64
	db.Model(&Invoice{}).Count(&totalInvoices)
	db.Model(&Invoice{}).Where("net_amount = 0 OR net_amount IS NULL").Count(&zeroAmountInvoices)

	report.DataQualityChecks = append(report.DataQualityChecks, createValidationResult(
		"Data Quality",
		"Invoices with valid amounts",
		totalInvoices,
		totalInvoices - zeroAmountInvoices,
		zeroAmountInvoices,
	))

	// 2. Customers with empty names
	var totalCustomers, emptyNameCustomers int64
	db.Model(&CustomerMaster{}).Count(&totalCustomers)
	db.Model(&CustomerMaster{}).Where("company_name = '' OR company_name IS NULL").Count(&emptyNameCustomers)

	report.DataQualityChecks = append(report.DataQualityChecks, createValidationResult(
		"Data Quality",
		"Customers with valid names",
		totalCustomers,
		totalCustomers - emptyNameCustomers,
		emptyNameCustomers,
	))

	// 3. Suppliers with empty names
	var totalSuppliers, emptyNameSuppliers int64
	db.Model(&SupplierMaster{}).Count(&totalSuppliers)
	db.Model(&SupplierMaster{}).Where("company_name = '' OR company_name IS NULL").Count(&emptyNameSuppliers)

	report.DataQualityChecks = append(report.DataQualityChecks, createValidationResult(
		"Data Quality",
		"Suppliers with valid names",
		totalSuppliers,
		totalSuppliers - emptyNameSuppliers,
		emptyNameSuppliers,
	))

	// 4. Invoices with future dates
	var futureDateInvoices int64
	db.Model(&Invoice{}).Where("invoice_date > ?", time.Now()).Count(&futureDateInvoices)

	report.DataQualityChecks = append(report.DataQualityChecks, createValidationResult(
		"Data Quality",
		"Invoices with valid dates (not future)",
		totalInvoices,
		totalInvoices - futureDateInvoices,
		futureDateInvoices,
	))

	// 5. Payments exceeding invoice amounts
	var totalPayments, excessivePayments int64
	db.Model(&Payment{}).Count(&totalPayments)
	db.Raw(`SELECT COUNT(*) FROM payments p
	        INNER JOIN invoices i ON p.invoice_id = i.id
	        WHERE p.amount > i.net_amount`).Scan(&excessivePayments)

	report.DataQualityChecks = append(report.DataQualityChecks, createValidationResult(
		"Data Quality",
		"Payments not exceeding invoice amounts",
		totalPayments,
		totalPayments - excessivePayments,
		excessivePayments,
	))

	// 6. Purchase Orders with zero amounts
	var totalPOs, zeroAmountPOs int64
	db.Model(&PurchaseOrder{}).Count(&totalPOs)
	db.Model(&PurchaseOrder{}).Where("total_amount = 0 OR total_amount IS NULL").Count(&zeroAmountPOs)

	report.DataQualityChecks = append(report.DataQualityChecks, createValidationResult(
		"Data Quality",
		"Purchase Orders with valid amounts",
		totalPOs,
		totalPOs - zeroAmountPOs,
		zeroAmountPOs,
	))

	return nil
}

// validateStatuses checks status field validity
func validateStatuses(db *gorm.DB, report *ValidationReport) error {
	fmt.Println("\n✓ Validating Status Fields...")

	validInvoiceStatuses := []string{"Draft", "Sent", "Paid", "Partially Paid", "Overdue", "Cancelled"}
	validOpportunityStages := []string{"Lead", "Qualified", "Proposal", "Negotiation", "Won", "Lost"}
	validPOStatuses := []string{"Draft", "Sent", "Acknowledged", "Partially Received", "Fully Received", "Cancelled"}

	// 1. Invoice statuses
	var totalInvoices, invalidInvoiceStatuses int64
	db.Model(&Invoice{}).Count(&totalInvoices)
	db.Model(&Invoice{}).Where("status NOT IN ?", validInvoiceStatuses).Count(&invalidInvoiceStatuses)

	report.StatusChecks = append(report.StatusChecks, createValidationResult(
		"Status Validation",
		"Invoice statuses valid",
		totalInvoices,
		totalInvoices - invalidInvoiceStatuses,
		invalidInvoiceStatuses,
	))

	// 2. Opportunity stages
	var totalOpportunities, invalidOpportunityStages int64
	db.Model(&Opportunity{}).Count(&totalOpportunities)
	db.Model(&Opportunity{}).Where("stage NOT IN ?", validOpportunityStages).Count(&invalidOpportunityStages)

	report.StatusChecks = append(report.StatusChecks, createValidationResult(
		"Status Validation",
		"Opportunity stages valid",
		totalOpportunities,
		totalOpportunities - invalidOpportunityStages,
		invalidOpportunityStages,
	))

	// 3. Purchase Order statuses
	var totalPOs, invalidPOStatuses int64
	db.Model(&PurchaseOrder{}).Count(&totalPOs)
	db.Model(&PurchaseOrder{}).Where("status NOT IN ?", validPOStatuses).Count(&invalidPOStatuses)

	report.StatusChecks = append(report.StatusChecks, createValidationResult(
		"Status Validation",
		"Purchase Order statuses valid",
		totalPOs,
		totalPOs - invalidPOStatuses,
		invalidPOStatuses,
	))

	return nil
}

// verifyTotals calculates and verifies key financial totals
func verifyTotals(db *gorm.DB, report *ValidationReport) error {
	fmt.Println("\n💰 Verifying Financial Totals...")

	// 1. 2025 Invoice Total
	var invoiceTotal2025 float64
	db.Model(&Invoice{}).
		Where("EXTRACT(YEAR FROM invoice_date) = 2025").
		Select("COALESCE(SUM(net_amount), 0)").
		Scan(&invoiceTotal2025)
	report.Totals["invoice_total_2025"] = invoiceTotal2025

	// 2. 2025 Payment Total
	var paymentTotal2025 float64
	db.Model(&Payment{}).
		Where("EXTRACT(YEAR FROM payment_date) = 2025").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&paymentTotal2025)
	report.Totals["payment_total_2025"] = paymentTotal2025

	// 3. 2025 Purchase Total
	var purchaseTotal2025 float64
	db.Model(&SupplierInvoice{}).
		Where("EXTRACT(YEAR FROM invoice_date) = 2025").
		Select("COALESCE(SUM(net_amount), 0)").
		Scan(&purchaseTotal2025)
	report.Totals["purchase_total_2025"] = purchaseTotal2025

	// 4. Count totals
	var customerCount, supplierCount, productCount int64
	db.Model(&CustomerMaster{}).Count(&customerCount)
	db.Model(&SupplierMaster{}).Count(&supplierCount)
	db.Model(&ProductMaster{}).Count(&productCount)

	report.Totals["customer_count"] = customerCount
	report.Totals["supplier_count"] = supplierCount
	report.Totals["product_count"] = productCount

	// 5. 2025 Record counts
	var invoiceCount2025, paymentCount2025, purchaseCount2025 int64
	db.Model(&Invoice{}).Where("EXTRACT(YEAR FROM invoice_date) = 2025").Count(&invoiceCount2025)
	db.Model(&Payment{}).Where("EXTRACT(YEAR FROM payment_date) = 2025").Count(&paymentCount2025)
	db.Model(&SupplierInvoice{}).Where("EXTRACT(YEAR FROM invoice_date) = 2025").Count(&purchaseCount2025)

	report.Totals["invoice_count_2025"] = invoiceCount2025
	report.Totals["payment_count_2025"] = paymentCount2025
	report.Totals["purchase_count_2025"] = purchaseCount2025

	// 6. Outstanding AR
	var outstandingAR float64
	db.Raw(`SELECT COALESCE(SUM(i.net_amount - COALESCE(p.paid, 0)), 0)
	        FROM invoices i
	        LEFT JOIN (
	            SELECT invoice_id, SUM(amount) as paid
	            FROM payments
	            GROUP BY invoice_id
	        ) p ON i.id = p.invoice_id
	        WHERE EXTRACT(YEAR FROM i.invoice_date) = 2025`).Scan(&outstandingAR)
	report.Totals["outstanding_ar"] = outstandingAR

	return nil
}

// createValidationResult creates a ValidationResult with status determination
func createValidationResult(category, checkName string, total, valid, invalid int64) ValidationResult {
	percentage := 0.0
	if total > 0 {
		percentage = (float64(valid) / float64(total)) * 100
	}

	status := "✅"
	details := ""

	if invalid > 0 {
		if percentage < 95 {
			status = "❌"
			details = fmt.Sprintf("%d orphaned records found", invalid)
		} else {
			status = "⚠️"
			details = fmt.Sprintf("%d issues found (acceptable)", invalid)
		}
	}

	return ValidationResult{
		Category:     category,
		CheckName:    checkName,
		TotalCount:   total,
		ValidCount:   valid,
		InvalidCount: invalid,
		Percentage:   percentage,
		Status:       status,
		Details:      details,
	}
}

// printValidationReport prints a comprehensive validation report
func printValidationReport(report *ValidationReport) {
	fmt.Println("\n" + colorCyan + "╔════════════════════════════════════════════════════════════╗" + colorReset)
	fmt.Println(colorCyan + "║           VALIDATION REPORT - 2025 DATA REBUILD           ║" + colorReset)
	fmt.Println(colorCyan + "╚════════════════════════════════════════════════════════════╝" + colorReset)

	fmt.Printf("\n📅 Generated: %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))

	// FK Integrity Checks
	fmt.Println("\n" + colorYellow + "═══ FK INTEGRITY CHECKS ═══" + colorReset)
	printValidationSection(report.FKChecks)

	// Data Quality Checks
	fmt.Println("\n" + colorYellow + "═══ DATA QUALITY CHECKS ═══" + colorReset)
	printValidationSection(report.DataQualityChecks)

	// Status Validation
	fmt.Println("\n" + colorYellow + "═══ STATUS VALIDATION ═══" + colorReset)
	printValidationSection(report.StatusChecks)

	// Totals Verification
	fmt.Println("\n" + colorYellow + "═══ TOTALS VERIFICATION (2025 DATA) ═══" + colorReset)
	fmt.Printf("\n📊 Financial Totals:\n")
	fmt.Printf("   Invoice Total:      BHD %14.3f  (%d invoices)\n",
		report.Totals["invoice_total_2025"],
		report.Totals["invoice_count_2025"])
	fmt.Printf("   Payment Received:   BHD %14.3f  (%d payments)\n",
		report.Totals["payment_total_2025"],
		report.Totals["payment_count_2025"])
	fmt.Printf("   Purchase Total:     BHD %14.3f  (%d supplier invoices)\n",
		report.Totals["purchase_total_2025"],
		report.Totals["purchase_count_2025"])
	fmt.Printf("   Outstanding AR:     BHD %14.3f\n", report.Totals["outstanding_ar"])

	fmt.Printf("\n📈 Master Data Counts:\n")
	fmt.Printf("   Customers:          %d\n", report.Totals["customer_count"])
	fmt.Printf("   Suppliers:          %d\n", report.Totals["supplier_count"])
	fmt.Printf("   Products:           %d\n", report.Totals["product_count"])

	// Calculate collection rate
	invoiceTotal := report.Totals["invoice_total_2025"].(float64)
	paymentTotal := report.Totals["payment_total_2025"].(float64)
	collectionRate := 0.0
	if invoiceTotal > 0 {
		collectionRate = (paymentTotal / invoiceTotal) * 100
	}
	fmt.Printf("\n💵 Collection Rate:    %.2f%%\n", collectionRate)
}

// printValidationSection prints a section of validation results
func printValidationSection(results []ValidationResult) {
	for _, result := range results {
		fmt.Printf("\n%s %s\n", result.Status, result.CheckName)
		fmt.Printf("   Total: %d | Valid: %d | Invalid: %d (%.2f%% valid)\n",
			result.TotalCount, result.ValidCount, result.InvalidCount, result.Percentage)

		if result.Details != "" {
			fmt.Printf("   ℹ️  %s\n", result.Details)
		}
	}
}
