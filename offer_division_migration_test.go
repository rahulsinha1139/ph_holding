package main

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateOfferDivisionSupportAddsAndBackfillsDivision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE offers (
			id TEXT PRIMARY KEY,
			offer_number TEXT,
			customer_name TEXT,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE orders (
			id TEXT PRIMARY KEY,
			offer_id TEXT,
			division TEXT,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE invoices (
			id TEXT PRIMARY KEY,
			offer_id TEXT,
			division TEXT,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`INSERT INTO offers (id, offer_number, customer_name) VALUES
			('offer-ph', '1-26', 'PH Customer'),
			('offer-ahs', '2-26', 'AHS Customer'),
			('offer-default', '3-26', 'Default Customer')`,
		`INSERT INTO orders (id, offer_id, division) VALUES ('order-1', 'offer-ahs', 'AHS Trading')`,
		`INSERT INTO invoices (id, offer_id, division) VALUES ('invoice-1', 'offer-ph', 'PH Trading')`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("failed to seed test db: %v", err)
		}
	}

	app := &App{db: db}
	app.migrateOfferDivisionSupport()

	if !app.hasColumn("offers", "division") {
		t.Fatalf("expected offers.division column to be created")
	}

	type resultRow struct {
		ID       string
		Division string
	}
	var rows []resultRow
	if err := db.Raw("SELECT id, division FROM offers ORDER BY id").Scan(&rows).Error; err != nil {
		t.Fatalf("failed to query offers divisions: %v", err)
	}

	got := map[string]string{}
	for _, row := range rows {
		got[row.ID] = row.Division
	}

	if got["offer-ahs"] != "AHS Trading" {
		t.Fatalf("offer-ahs division = %q, want %q", got["offer-ahs"], "AHS Trading")
	}
	if got["offer-ph"] != "PH Trading" {
		t.Fatalf("offer-ph division = %q, want %q", got["offer-ph"], "PH Trading")
	}
	if got["offer-default"] != "PH Trading" {
		t.Fatalf("offer-default division = %q, want %q", got["offer-default"], "PH Trading")
	}
}
