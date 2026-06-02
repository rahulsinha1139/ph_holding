//go:build ignore

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"html"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xuri/excelize/v2"
)

const (
	seedBy   = "2025_tally_seed"
	division = "PH Trading"
)

type salesInvoice struct {
	Date           time.Time
	CustomerName   string
	TRN            string
	VoucherType    string
	InvoiceNumber  string
	Reference      string
	TaxableValue   float64
	VATAmount      float64
	GrandTotal     float64
	CustomerID     string
	OutstandingBHD float64
	Status         string
	OpeningAR      bool
}

type purchaseInvoice struct {
	Date          time.Time
	SupplierName  string
	VoucherType   string
	VoucherNumber string
	InvoiceNumber string
	SubtotalBHD   float64
	VATBHD        float64
	TotalBHD      float64
	SupplierID    string
}

type arBalance struct {
	CustomerName string
	Debit        float64
	Credit       float64
	Less15       float64
	Days16To30   float64
	Days31To45   float64
	Days46To90   float64
	Days91Plus   float64
	Total        float64
	CustomerID   string
}

type entity struct {
	ID   string
	Name string
	Code string
	Tax  string
}

type plan struct {
	SalesInvoices     []salesInvoice
	PurchaseInvoices  []purchaseInvoice
	ARBalances        []arBalance
	CustomersTouched  map[string]string
	SuppliersTouched  map[string]string
	SalesTaxableTotal float64
	SalesVATTotal     float64
	SalesGrandTotal   float64
	PurchaseSubtotal  float64
	PurchaseVAT       float64
	PurchaseTotal     float64
	AROutstanding     float64
	PaymentTotal      float64
	OpeningARTotal    float64
	OpeningARCount    int
}

func main() {
	var inputDir string
	var envPath string
	var apply bool
	var parseOnly bool
	var verifyOnly bool
	flag.StringVar(&inputDir, "input-dir", "/Users/rahulsinha/Downloads/OneDrive_1_14-5-2026", "directory containing the 2025 Tally export workbooks")
	flag.StringVar(&envPath, "env", "deploy_package/AsymmFlow_Portable_2026_05_14_PH_TRADING/.env", "environment file with Supabase DB settings")
	flag.BoolVar(&apply, "apply", false, "write changes to Supabase; default is dry run")
	flag.BoolVar(&parseOnly, "parse-only", false, "parse source workbooks without connecting to Supabase")
	flag.BoolVar(&verifyOnly, "verify-only", false, "connect to Supabase and print aggregate totals for the seeded rows")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	seedPlan, err := buildPlan(inputDir)
	if err != nil {
		log.Fatal(err)
	}

	printPlan(seedPlan)
	if parseOnly {
		return
	}

	env, err := readEnv(envPath)
	if err != nil {
		log.Fatal(err)
	}
	db, err := openSupabase(ctx, env)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := remotePreflight(ctx, db); err != nil {
		log.Fatal(err)
	}
	if verifyOnly {
		if err := remoteSeedReport(ctx, db); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := hydrateRemoteEntities(ctx, db, seedPlan); err != nil {
		log.Fatal(err)
	}
	allocateAR(seedPlan)
	printAllocation(seedPlan)

	if !apply {
		fmt.Println("\nDRY RUN ONLY: remote connection and mapping succeeded. Re-run with --apply to seed Supabase.")
		return
	}

	if err := applyPlan(ctx, db, seedPlan); err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nAPPLIED: 2025 Tally exports seeded/upserted into Supabase.")
}

func buildPlan(inputDir string) (*plan, error) {
	sales, err := parseSalesInvoices(filepath.Join(inputDir, "invoices 2025.xlsx"))
	if err != nil {
		return nil, err
	}
	purchases, err := parsePurchaseInvoices(filepath.Join(inputDir, "purchase 2025.xlsx"))
	if err != nil {
		return nil, err
	}
	ar, err := parseARBalances(filepath.Join(inputDir, "AR payment defaulters.xlsx"))
	if err != nil {
		return nil, err
	}
	p := &plan{
		SalesInvoices:    sales,
		PurchaseInvoices: purchases,
		ARBalances:       ar,
		CustomersTouched: make(map[string]string),
		SuppliersTouched: make(map[string]string),
	}
	for i := range p.SalesInvoices {
		p.SalesTaxableTotal += p.SalesInvoices[i].TaxableValue
		p.SalesVATTotal += p.SalesInvoices[i].VATAmount
		p.SalesGrandTotal += p.SalesInvoices[i].GrandTotal
	}
	for i := range p.PurchaseInvoices {
		p.PurchaseSubtotal += p.PurchaseInvoices[i].SubtotalBHD
		p.PurchaseVAT += p.PurchaseInvoices[i].VATBHD
		p.PurchaseTotal += p.PurchaseInvoices[i].TotalBHD
	}
	for i := range p.ARBalances {
		p.AROutstanding += p.ARBalances[i].Total
	}
	return p, nil
}

func parseSalesInvoices(path string) ([]salesInvoice, error) {
	rows, err := workbookRows(path, "Voucher Register")
	if err != nil {
		return nil, err
	}
	var out []salesInvoice
	for _, row := range rows {
		if len(row) < 8 {
			continue
		}
		date, ok := parseDate(row[0])
		if !ok {
			continue
		}
		customer := clean(row[1])
		if customer == "" || strings.EqualFold(customer, "Grand Total") {
			continue
		}
		taxable := money(row[6])
		vat := money(row[7])
		inv := salesInvoice{
			Date:          date,
			CustomerName:  customer,
			TRN:           clean(row[2]),
			VoucherType:   clean(row[3]),
			InvoiceNumber: clean(row[4]),
			Reference:     clean(row[5]),
			TaxableValue:  round3(taxable),
			VATAmount:     round3(vat),
			GrandTotal:    round3(taxable + vat),
			Status:        "Paid",
		}
		if inv.InvoiceNumber == "" {
			return nil, fmt.Errorf("sales invoice for %s on %s has blank invoice number", inv.CustomerName, inv.Date.Format("2006-01-02"))
		}
		out = append(out, inv)
	}
	return out, nil
}

func parsePurchaseInvoices(path string) ([]purchaseInvoice, error) {
	rows, err := workbookRows(path, "Purchase Register")
	if err != nil {
		return nil, err
	}
	var out []purchaseInvoice
	for _, row := range rows {
		if len(row) < 6 {
			continue
		}
		date, ok := parseDate(row[0])
		if !ok {
			continue
		}
		supplier := clean(row[1])
		if supplier == "" || strings.EqualFold(supplier, "Total:") {
			continue
		}
		subtotal := money(row[5])
		if subtotal == 0 {
			continue
		}
		vt := clean(row[2])
		vat := 0.0
		if strings.Contains(strings.ToLower(vt), "10%") || strings.EqualFold(vt, "LOCAL PURCHASE") || strings.EqualFold(vt, "Purchase") {
			vat = round3(subtotal * 0.10)
		}
		vno := clean(row[3])
		invoiceNo := fmt.Sprintf("PUR25-%s-%s-%s", date.Format("20060102"), slug(vt, 10), slug(vno, 12))
		out = append(out, purchaseInvoice{
			Date:          date,
			SupplierName:  supplier,
			VoucherType:   vt,
			VoucherNumber: vno,
			InvoiceNumber: invoiceNo,
			SubtotalBHD:   round3(subtotal),
			VATBHD:        round3(vat),
			TotalBHD:      round3(subtotal + vat),
		})
	}
	return out, nil
}

func parseARBalances(path string) ([]arBalance, error) {
	rows, err := workbookRows(path, "Trade Debtors")
	if err != nil {
		return nil, err
	}
	var out []arBalance
	for _, row := range rows {
		if len(row) < 9 {
			continue
		}
		name := clean(row[0])
		if name == "" || strings.EqualFold(name, "Particulars") || strings.EqualFold(name, "Grand Total") {
			continue
		}
		total := money(row[8])
		if total == 0 {
			continue
		}
		out = append(out, arBalance{
			CustomerName: name,
			Debit:        round3(money(row[1])),
			Credit:       round3(money(row[2])),
			Less15:       round3(money(row[3])),
			Days16To30:   round3(money(row[4])),
			Days31To45:   round3(money(row[5])),
			Days46To90:   round3(money(row[6])),
			Days91Plus:   round3(money(row[7])),
			Total:        round3(total),
		})
	}
	return out, nil
}

func workbookRows(path, sheet string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read sheet %s in %s: %w", sheet, path, err)
	}
	return rows, nil
}

func readEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read env %s: %w", path, err)
	}
	env := map[string]string{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		env[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return env, nil
}

func openSupabase(ctx context.Context, env map[string]string) (*sql.DB, error) {
	cfg := url.Values{}
	cfg.Set("sslmode", envDefault(env, "SUPABASE_DB_SSLMODE", "require"))
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s",
		url.QueryEscape(env["SUPABASE_DB_USER"]),
		url.QueryEscape(env["SUPABASE_DB_PASSWORD"]),
		env["SUPABASE_DB_HOST"],
		envDefault(env, "SUPABASE_DB_PORT", "5432"),
		envDefault(env, "SUPABASE_DB_NAME", "postgres"),
		cfg.Encode(),
	)
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(4)
	conn.SetConnMaxLifetime(5 * time.Minute)
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connect to Supabase: %w", err)
	}
	return conn, nil
}

func remotePreflight(ctx context.Context, db *sql.DB) error {
	required := []string{
		"customers", "suppliers", "invoices", "invoice_items", "payments",
		"supplier_invoices", "supplier_invoice_items", "entity_notes",
	}
	for _, table := range required {
		var exists bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("remote table %q is missing", table)
		}
	}
	var invoiceCount, supplierInvoiceCount, paymentCount int
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM invoices WHERE deleted_at IS NULL").Scan(&invoiceCount)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM supplier_invoices WHERE deleted_at IS NULL").Scan(&supplierInvoiceCount)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM payments WHERE deleted_at IS NULL").Scan(&paymentCount)
	fmt.Printf("\nRemote preflight OK: invoices=%d supplier_invoices=%d payments=%d\n", invoiceCount, supplierInvoiceCount, paymentCount)
	return nil
}

func remoteSeedReport(ctx context.Context, db *sql.DB) error {
	fmt.Println("\nSeed verification:")
	type row struct {
		Label string
		Count int
		A     float64
		B     float64
		C     float64
		D     float64
	}
	queries := []struct {
		label string
		sql   string
	}{
		{"customer invoices", `SELECT count(*), COALESCE(sum(subtotal_bhd),0), COALESCE(sum(vatbhd),0), COALESCE(sum(grand_total_bhd),0), COALESCE(sum(outstanding_bhd),0) FROM invoices WHERE (created_by = $1 OR updated_by = $1) AND deleted_at IS NULL`},
		{"customer payments", `SELECT count(*), COALESCE(sum(amount_bhd),0), 0, 0, 0 FROM payments WHERE created_by = $1 AND deleted_at IS NULL`},
		{"supplier invoices", `SELECT count(*), COALESCE(sum(subtotal_bhd),0), COALESCE(sum(vatbhd),0), COALESCE(sum(total_bhd),0), 0 FROM supplier_invoices WHERE (created_by = $1 OR updated_by = $1) AND deleted_at IS NULL`},
		{"supplier invoice items", `SELECT count(*), COALESCE(sum(total_price),0), 0, 0, 0 FROM supplier_invoice_items WHERE created_by = $1 AND deleted_at IS NULL`},
		{"AR notes", `SELECT count(*), 0, 0, 0, 0 FROM entity_notes WHERE created_by = $1 AND note_type = 'payment' AND deleted_at IS NULL`},
	}
	for _, q := range queries {
		var r row
		r.Label = q.label
		if err := db.QueryRowContext(ctx, q.sql, seedBy).Scan(&r.Count, &r.A, &r.B, &r.C, &r.D); err != nil {
			return fmt.Errorf("verify %s: %w", q.label, err)
		}
		switch q.label {
		case "customer invoices":
			fmt.Printf("- %s: %d | subtotal %.3f | VAT %.3f | gross %.3f | outstanding %.3f\n", r.Label, r.Count, r.A, r.B, r.C, r.D)
		case "supplier invoices":
			fmt.Printf("- %s: %d | subtotal %.3f | VAT %.3f | total %.3f\n", r.Label, r.Count, r.A, r.B, r.C)
		default:
			fmt.Printf("- %s: %d | amount %.3f\n", r.Label, r.Count, r.A)
		}
	}
	return nil
}

func hydrateRemoteEntities(ctx context.Context, db *sql.DB, p *plan) error {
	customers, err := loadEntities(ctx, db, "customers", "business_name", "customer_code", "trn")
	if err != nil {
		return err
	}
	suppliers, err := loadEntities(ctx, db, "suppliers", "supplier_name", "supplier_code", "tax_id")
	if err != nil {
		return err
	}
	customerIndex := indexEntities(customers)
	supplierIndex := indexEntities(suppliers)
	usedCustomerCodes := usedCodes(customers)
	usedSupplierCodes := usedCodes(suppliers)

	for i := range p.SalesInvoices {
		e, ok := findEntity(customerIndex, p.SalesInvoices[i].CustomerName, p.SalesInvoices[i].TRN)
		if !ok {
			code := nextCode("CUST25", p.SalesInvoices[i].CustomerName, usedCustomerCodes)
			e = entity{ID: uuid.NewString(), Name: p.SalesInvoices[i].CustomerName, Code: code, Tax: p.SalesInvoices[i].TRN}
			customerIndex[normName(e.Name)] = e
		}
		p.SalesInvoices[i].CustomerID = e.ID
		p.CustomersTouched[e.ID] = e.Name
	}
	for i := range p.ARBalances {
		e, ok := findEntity(customerIndex, p.ARBalances[i].CustomerName, "")
		if !ok {
			code := nextCode("CUST25", p.ARBalances[i].CustomerName, usedCustomerCodes)
			e = entity{ID: uuid.NewString(), Name: p.ARBalances[i].CustomerName, Code: code}
			customerIndex[normName(e.Name)] = e
		}
		p.ARBalances[i].CustomerID = e.ID
		p.CustomersTouched[e.ID] = e.Name
	}
	for i := range p.PurchaseInvoices {
		e, ok := findEntity(supplierIndex, p.PurchaseInvoices[i].SupplierName, "")
		if !ok {
			code := nextCode("SUP25", p.PurchaseInvoices[i].SupplierName, usedSupplierCodes)
			e = entity{ID: uuid.NewString(), Name: p.PurchaseInvoices[i].SupplierName, Code: code}
			supplierIndex[normName(e.Name)] = e
		}
		p.PurchaseInvoices[i].SupplierID = e.ID
		p.SuppliersTouched[e.ID] = e.Name
	}
	return nil
}

func loadEntities(ctx context.Context, db *sql.DB, table, nameCol, codeCol, taxCol string) ([]entity, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT id, COALESCE(%s,''), COALESCE(%s,''), COALESCE(%s,'') FROM %s WHERE deleted_at IS NULL", nameCol, codeCol, taxCol, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity
	for rows.Next() {
		var e entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Code, &e.Tax); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func indexEntities(entities []entity) map[string]entity {
	idx := map[string]entity{}
	for _, e := range entities {
		if n := normName(e.Name); n != "" {
			idx[n] = e
		}
		if t := strings.TrimSpace(e.Tax); t != "" {
			idx["tax:"+t] = e
		}
	}
	return idx
}

func usedCodes(entities []entity) map[string]bool {
	used := map[string]bool{}
	for _, e := range entities {
		if e.Code != "" {
			used[e.Code] = true
		}
	}
	return used
}

func findEntity(idx map[string]entity, name, tax string) (entity, bool) {
	if strings.TrimSpace(tax) != "" {
		if e, ok := idx["tax:"+strings.TrimSpace(tax)]; ok {
			return e, true
		}
	}
	e, ok := idx[normName(name)]
	return e, ok
}

func allocateAR(p *plan) {
	targetByCustomer := map[string]float64{}
	nameByCustomer := map[string]string{}
	for _, ar := range p.ARBalances {
		targetByCustomer[ar.CustomerID] += ar.Total
		nameByCustomer[ar.CustomerID] = ar.CustomerName
	}
	indicesByCustomer := map[string][]int{}
	for i := range p.SalesInvoices {
		indicesByCustomer[p.SalesInvoices[i].CustomerID] = append(indicesByCustomer[p.SalesInvoices[i].CustomerID], i)
	}
	for customerID := range targetByCustomer {
		indices := indicesByCustomer[customerID]
		sort.Slice(indices, func(a, b int) bool {
			return p.SalesInvoices[indices[a]].Date.After(p.SalesInvoices[indices[b]].Date)
		})
		remaining := round3(targetByCustomer[customerID])
		for _, idx := range indices {
			inv := &p.SalesInvoices[idx]
			outstanding := 0.0
			if remaining > 0 {
				outstanding = math.Min(inv.GrandTotal, remaining)
				remaining = round3(remaining - outstanding)
			}
			inv.OutstandingBHD = round3(outstanding)
			switch {
			case inv.OutstandingBHD <= 0.001:
				inv.Status = "Paid"
			default:
				inv.Status = "Overdue"
			}
		}
		if remaining > 0.001 {
			opening := salesInvoice{
				Date:           time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
				CustomerName:   nameByCustomer[customerID],
				InvoiceNumber:  "AROPEN-2025-" + slug(nameByCustomer[customerID], 24),
				Reference:      "Opening AR balance from 2025 Trade Debtors aging",
				TaxableValue:   round3(remaining),
				VATAmount:      0,
				GrandTotal:     round3(remaining),
				CustomerID:     customerID,
				OutstandingBHD: round3(remaining),
				Status:         "Overdue",
				OpeningAR:      true,
			}
			p.SalesInvoices = append(p.SalesInvoices, opening)
			p.OpeningARTotal += opening.GrandTotal
			p.OpeningARCount++
		}
	}
	p.PaymentTotal = 0
	for i := range p.SalesInvoices {
		p.PaymentTotal += round3(p.SalesInvoices[i].GrandTotal - p.SalesInvoices[i].OutstandingBHD)
	}
	p.PaymentTotal = round3(p.PaymentTotal)
	p.OpeningARTotal = round3(p.OpeningARTotal)
}

func applyPlan(ctx context.Context, db *sql.DB, p *plan) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM entity_notes WHERE created_by = $1 AND note_type = 'payment'", seedBy); err != nil {
		return err
	}

	if err := upsertCustomers(ctx, tx, p); err != nil {
		return err
	}
	if err := upsertSuppliers(ctx, tx, p); err != nil {
		return err
	}
	if err := upsertSalesInvoices(ctx, tx, p); err != nil {
		return err
	}
	if err := upsertSupplierInvoices(ctx, tx, p); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func upsertCustomers(ctx context.Context, tx *sql.Tx, p *plan) error {
	arByCustomer := map[string]arBalance{}
	for _, ar := range p.ARBalances {
		arByCustomer[ar.CustomerID] = ar
	}
	for id, name := range p.CustomersTouched {
		ar := arByCustomer[id]
		risk := "Low"
		overdueDays := 0
		if ar.Total > 0 {
			risk = "High"
			overdueDays = 91
			if ar.Days91Plus == 0 {
				overdueDays = 46
			}
			if ar.Days46To90 == 0 && ar.Days91Plus == 0 {
				overdueDays = 31
			}
			if ar.Days31To45 == 0 && ar.Days46To90 == 0 && ar.Days91Plus == 0 {
				overdueDays = 16
			}
			if ar.Days16To30 == 0 && ar.Days31To45 == 0 && ar.Days46To90 == 0 && ar.Days91Plus == 0 {
				overdueDays = 15
				risk = "Medium"
			}
		}
		code := "CUST25-" + slug(name, 24)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO customers (
				id, created_at, updated_at, version, created_by,
				customer_id, customer_code, business_name, trading_name, customer_type,
				country, status, payment_grade, customer_grade, payment_terms_days,
				outstanding_bhd, ar_risk_tier, overdue_days
			) VALUES ($1, now(), now(), 1, $2, $3, $3, $4, $4, 'Customer', 'Bahrain', 'Active', 'C', 'C', 30, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				updated_at = now(),
				business_name = EXCLUDED.business_name,
				trading_name = EXCLUDED.trading_name,
				status = 'Active',
				outstanding_bhd = EXCLUDED.outstanding_bhd,
				ar_risk_tier = EXCLUDED.ar_risk_tier,
				overdue_days = EXCLUDED.overdue_days`,
			id, seedBy, code, name, ar.Total, risk, overdueDays)
		if err != nil {
			return fmt.Errorf("upsert customer %s: %w", name, err)
		}
		if ar.Total > 0 {
			note := fmt.Sprintf("2025 Tally AR aging imported: total %.3f BHD; less than 15 days %.3f; 16-30 days %.3f; 31-45 days %.3f; 46-90 days %.3f; >91 days %.3f.",
				ar.Total, ar.Less15, ar.Days16To30, ar.Days31To45, ar.Days46To90, ar.Days91Plus)
			_, err = tx.ExecContext(ctx, `
				INSERT INTO entity_notes (id, created_at, updated_at, version, created_by, entity_type, entity_id, note_type, content)
				VALUES ($1, now(), now(), 1, $2, 'customer', $3, 'payment', $4)`,
				uuid.NewString(), seedBy, id, note)
			if err != nil {
				return fmt.Errorf("insert AR note for %s: %w", name, err)
			}
		}
	}
	return nil
}

func upsertSuppliers(ctx context.Context, tx *sql.Tx, p *plan) error {
	for id, name := range p.SuppliersTouched {
		code := "SUP25-" + slug(name, 24)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO suppliers (
				id, created_at, updated_at, version, created_by,
				supplier_code, supplier_name, country, supplier_type, rating, payment_terms
			) VALUES ($1, now(), now(), 1, $2, $3, $4, 'Bahrain', 'Supplier', 3, 'Net 30')
			ON CONFLICT (id) DO UPDATE SET
				updated_at = now(),
				supplier_name = EXCLUDED.supplier_name`,
			id, seedBy, code, name)
		if err != nil {
			return fmt.Errorf("upsert supplier %s: %w", name, err)
		}
	}
	return nil
}

func upsertSalesInvoices(ctx context.Context, tx *sql.Tx, p *plan) error {
	for _, inv := range p.SalesInvoices {
		invoiceID := uuid.NewString()
		paid := round3(inv.GrandTotal - inv.OutstandingBHD)
		due := inv.Date.AddDate(0, 0, 30)
		err := tx.QueryRowContext(ctx, `
			INSERT INTO invoices (
				id, created_at, updated_at, version, created_by,
				invoice_number, invoice_date, due_date, customer_id, customer_name,
				customer_po_number, customer_reference, delivery_note_ref, other_references,
				subtotal_bhd, vatbhd, vat_percent, grand_total_bhd, outstanding_bhd,
				status, payment_terms, issued_by, division, updated_by
			) VALUES (
				$1, now(), now(), 1, $2,
				$3, $4, $5, $6, $7,
				$8, $8, $8, $8,
				$9, $10, $11, $12, $13,
				$14, '30 days', 'Tally 2025 Import', $15, $2
			)
			ON CONFLICT (invoice_number) DO UPDATE SET
				updated_at = now(),
				invoice_date = EXCLUDED.invoice_date,
				due_date = EXCLUDED.due_date,
				customer_id = EXCLUDED.customer_id,
				customer_name = EXCLUDED.customer_name,
				customer_po_number = EXCLUDED.customer_po_number,
				customer_reference = EXCLUDED.customer_reference,
				subtotal_bhd = EXCLUDED.subtotal_bhd,
				vatbhd = EXCLUDED.vatbhd,
				vat_percent = EXCLUDED.vat_percent,
				grand_total_bhd = EXCLUDED.grand_total_bhd,
				outstanding_bhd = EXCLUDED.outstanding_bhd,
				status = EXCLUDED.status,
				division = EXCLUDED.division,
				updated_by = EXCLUDED.updated_by
			RETURNING id`,
			invoiceID, seedBy, inv.InvoiceNumber, inv.Date, due, inv.CustomerID, inv.CustomerName,
			inv.Reference, inv.TaxableValue, inv.VATAmount, vatPercent(inv.TaxableValue, inv.VATAmount),
			inv.GrandTotal, inv.OutstandingBHD, inv.Status, division).Scan(&invoiceID)
		if err != nil {
			return fmt.Errorf("upsert invoice %s: %w", inv.InvoiceNumber, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM invoice_items WHERE invoice_id = $1 AND created_by = $2", invoiceID, seedBy); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO invoice_items (
				id, created_at, updated_at, version, created_by, invoice_id,
				line_number, description, quantity, rate, total_bhd,
				equipment, specification, currency, total_price
			) VALUES ($1, now(), now(), 1, $2, $3, 1, $4, 1, $5, $5, $4, $4, 'BHD', $5)`,
			uuid.NewString(), seedBy, invoiceID, "Imported 2025 sales invoice "+inv.Reference, inv.TaxableValue); err != nil {
			return fmt.Errorf("insert invoice item %s: %w", inv.InvoiceNumber, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM payments WHERE invoice_id = $1 AND created_by = $2", invoiceID, seedBy); err != nil {
			return err
		}
		if paid > 0.001 {
			paymentDate := inv.Date.AddDate(0, 0, 30)
			if paymentDate.After(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)) {
				paymentDate = time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
			}
			days := int(paymentDate.Sub(inv.Date).Hours() / 24)
			if days < 0 {
				days = 0
			}
			key := idempotency(invoiceID, paid, paymentDate, "2025 seed settlement")
			_, err := tx.ExecContext(ctx, `
				INSERT INTO payments (
					id, created_at, updated_at, version, created_by,
					invoice_id, invoice_number, amount_bhd, payment_date, payment_method,
					days_to_payment, idempotency_key, reference, division, updated_by
				) VALUES ($1, now(), now(), 1, $2, $3, $4, $5, $6, 'Bank Transfer', $7, $8, '2025 seed settlement', $9, $2)
				ON CONFLICT (idempotency_key) DO UPDATE SET
					updated_at = now(),
					amount_bhd = EXCLUDED.amount_bhd,
					payment_date = EXCLUDED.payment_date,
					days_to_payment = EXCLUDED.days_to_payment`,
				uuid.NewString(), seedBy, invoiceID, inv.InvoiceNumber, paid, paymentDate, days, key, division)
			if err != nil {
				return fmt.Errorf("insert payment for %s: %w", inv.InvoiceNumber, err)
			}
		}
	}
	return nil
}

func upsertSupplierInvoices(ctx context.Context, tx *sql.Tx, p *plan) error {
	for _, inv := range p.PurchaseInvoices {
		invoiceID := uuid.NewString()
		due := inv.Date.AddDate(0, 0, 30)
		err := tx.QueryRowContext(ctx,
			"SELECT id FROM supplier_invoices WHERE invoice_number = $1 AND created_by = $2 AND deleted_at IS NULL LIMIT 1",
			inv.InvoiceNumber, seedBy).Scan(&invoiceID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("lookup supplier invoice %s: %w", inv.InvoiceNumber, err)
		}
		notes := fmt.Sprintf("Imported from 2025 purchase register: %s voucher %s", inv.VoucherType, inv.VoucherNumber)
		if err == sql.ErrNoRows {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO supplier_invoices (
					id, created_at, updated_at, version, created_by,
					invoice_number, invoice_date, due_date, supplier_id, supplier_name,
					subtotal_bhd, vatbhd, total_bhd, status, payment_status, payment_date,
					currency, exchange_rate, subtotal_foreign, vat_foreign, total_foreign,
					match_status, po_match_ok, grn_match_ok, approved_by, approved_at,
					payment_method, payment_ref, division, updated_by, notes
				) VALUES (
					$1, now(), now(), 1, $2,
					$3, $4, $5, $6, $7,
					$8, $9, $10, 'Paid', 'Paid', $4,
					'BHD', 1, $8, $9, $10,
					'Matched', true, true, 'Tally 2025 Import', now(),
					'Bank Transfer', '2025 seed settlement', $11, $2, $12
				)`,
				invoiceID, seedBy, inv.InvoiceNumber, inv.Date, due, inv.SupplierID, inv.SupplierName,
				inv.SubtotalBHD, inv.VATBHD, inv.TotalBHD, division, notes)
		} else {
			_, err = tx.ExecContext(ctx, `
				UPDATE supplier_invoices SET
					updated_at = now(),
					invoice_date = $2,
					due_date = $3,
					supplier_id = $4,
					supplier_name = $5,
					subtotal_bhd = $6,
					vatbhd = $7,
					total_bhd = $8,
					status = 'Paid',
					payment_status = 'Paid',
					payment_date = $2,
					currency = 'BHD',
					exchange_rate = 1,
					subtotal_foreign = $6,
					vat_foreign = $7,
					total_foreign = $8,
					match_status = 'Matched',
					po_match_ok = true,
					grn_match_ok = true,
					payment_method = 'Bank Transfer',
					payment_ref = '2025 seed settlement',
					division = $9,
					updated_by = $10,
					notes = $11
				WHERE id = $1`,
				invoiceID, inv.Date, due, inv.SupplierID, inv.SupplierName,
				inv.SubtotalBHD, inv.VATBHD, inv.TotalBHD, division, seedBy, notes)
		}
		if err != nil {
			return fmt.Errorf("upsert supplier invoice %s: %w", inv.InvoiceNumber, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM supplier_invoice_items WHERE supplier_invoice_id = $1 AND created_by = $2", invoiceID, seedBy); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO supplier_invoice_items (
				id, created_at, updated_at, version, created_by, supplier_invoice_id,
				line_number, description, quantity, unit_price, total_price, currency
			) VALUES ($1, now(), now(), 1, $2, $3, 1, $4, 1, $5, $5, 'BHD')`,
			uuid.NewString(), seedBy, invoiceID, inv.VoucherType+" voucher "+inv.VoucherNumber, inv.SubtotalBHD); err != nil {
			return fmt.Errorf("insert supplier invoice item %s: %w", inv.InvoiceNumber, err)
		}
	}
	return nil
}

func printPlan(p *plan) {
	fmt.Printf("Parsed source workbooks:\n")
	fmt.Printf("- Sales invoices: %d | taxable %.3f BHD | VAT %.3f BHD | gross %.3f BHD\n", len(p.SalesInvoices), p.SalesTaxableTotal, p.SalesVATTotal, p.SalesGrandTotal)
	fmt.Printf("- Purchase invoices: %d | subtotal %.3f BHD | estimated VAT %.3f BHD | total %.3f BHD\n", len(p.PurchaseInvoices), p.PurchaseSubtotal, p.PurchaseVAT, p.PurchaseTotal)
	fmt.Printf("- AR customer balances: %d | outstanding %.3f BHD\n", len(p.ARBalances), p.AROutstanding)
}

func printAllocation(p *plan) {
	statusCount := map[string]int{}
	outstanding := 0.0
	for _, inv := range p.SalesInvoices {
		statusCount[inv.Status]++
		outstanding += inv.OutstandingBHD
	}
	fmt.Printf("\nRemote mapping/allocation:\n")
	fmt.Printf("- Customers touched: %d\n", len(p.CustomersTouched))
	fmt.Printf("- Suppliers touched: %d\n", len(p.SuppliersTouched))
	fmt.Printf("- Invoice statuses: Paid=%d PartiallyPaid=%d Overdue=%d Sent=%d\n", statusCount["Paid"], statusCount["PartiallyPaid"], statusCount["Overdue"], statusCount["Sent"])
	fmt.Printf("- Allocated AR outstanding: %.3f BHD\n", round3(outstanding))
	if p.OpeningARCount > 0 {
		fmt.Printf("- Opening AR balance invoices added: %d | %.3f BHD (dated 2024-12-31, not 2025 revenue)\n", p.OpeningARCount, p.OpeningARTotal)
	}
	fmt.Printf("- Seeded customer payment total: %.3f BHD\n", p.PaymentTotal)
}

func envDefault(env map[string]string, key, fallback string) string {
	if strings.TrimSpace(env[key]) == "" {
		return fallback
	}
	return env[key]
}

func parseDate(raw string) (time.Time, bool) {
	s := clean(raw)
	if s == "" {
		return time.Time{}, false
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 20000 && f < 80000 {
		t, err := excelize.ExcelDateToTime(f, false)
		return t, err == nil
	}
	layouts := []string{
		"2006-01-02 15:04:05", "2006-01-02", "2-Jan-2006", "02-Jan-2006",
		"2-Jan-06", "02-Jan-06", "1/2/2006", "02/01/2006", "1/2/06", "02/01/06",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func clean(raw string) string {
	s := html.UnescapeString(raw)
	s = strings.ReplaceAll(s, "_x000D_", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
	return s
}

func money(raw string) float64 {
	s := strings.TrimSpace(clean(raw))
	if s == "" || s == "-" {
		return 0
	}
	negative := strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
	s = strings.Trim(s, "()")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	if negative {
		v = -v
	}
	return v
}

func normName(name string) string {
	s := strings.ToUpper(clean(name))
	s = strings.ReplaceAll(s, "&", "AND")
	s = regexp.MustCompile(`\bW\s*\.?\s*L\s*\.?\s*L\.?\b`).ReplaceAllString(s, "WLL")
	s = regexp.MustCompile(`\bB\s*\.?\s*S\s*\.?\s*C\.?\b`).ReplaceAllString(s, "BSC")
	s = regexp.MustCompile(`[^A-Z0-9]+`).ReplaceAllString(s, " ")
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

func slug(value string, max int) string {
	s := normName(value)
	if s == "" {
		s = "NA"
	}
	s = strings.ReplaceAll(s, " ", "-")
	if len(s) > max {
		s = strings.Trim(s[:max], "-")
	}
	return s
}

func nextCode(prefix, name string, used map[string]bool) string {
	base := prefix + "-" + slug(name, 24)
	code := base
	for i := 2; used[code]; i++ {
		code = fmt.Sprintf("%s-%02d", base, i)
	}
	used[code] = true
	return code
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func vatPercent(subtotal, vat float64) float64 {
	if subtotal <= 0 || vat <= 0 {
		return 0
	}
	return round3(vat / subtotal * 100)
}

func idempotency(invoiceID string, amount float64, date time.Time, ref string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%.3f|%s|%s", invoiceID, amount, date.Format("2006-01-02"), ref)))
	return hex.EncodeToString(sum[:])
}
