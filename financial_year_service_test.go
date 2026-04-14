package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetAvailableFinancialYearsExcludesOfferOnlyYears(t *testing.T) {
	app := setupTestApp(t)

	offerYear := 2027
	require.NoError(t, app.db.Create(&Offer{
		Base:           Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:    "OFF-2027-001",
		QuotationDate:  time.Date(offerYear, time.January, 15, 0, 0, 0, 0, time.UTC),
		ValidityDate:   time.Date(offerYear, time.February, 15, 0, 0, 0, 0, time.UTC),
		CustomerID:     uuid.New().String(),
		CustomerName:   "Offer Only Customer",
		TotalValueBHD:  1000,
		Stage:          "Quoted",
		RevisionNumber: 1,
		Division:       "PH Trading",
	}).Error)

	years, err := app.GetAvailableFinancialYears()
	require.NoError(t, err)
	for _, year := range years {
		if year == offerYear {
			t.Fatalf("offer-only year %d should not appear in financial dashboard years: %v", offerYear, years)
		}
	}
}

func TestGetDynamicFinancialDashboardExcludesDraftAndProformaFromReceivables(t *testing.T) {
	app := setupTestApp(t)
	customerID := seedTestCustomer(t, app.db, "Finance Test Customer")
	year := 2026
	invoiceDate := time.Date(year, time.January, 10, 0, 0, 0, 0, time.UTC)
	dueDate := invoiceDate.AddDate(0, 0, 30)

	invoices := []Invoice{
		{
			Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
			InvoiceNumber:  "INV-SENT-001",
			InvoiceDate:    invoiceDate,
			DueDate:        dueDate,
			CustomerID:     customerID,
			CustomerName:   "Finance Test Customer",
			Status:         "Sent",
			GrandTotalBHD:  100,
			OutstandingBHD: 100,
			SubtotalBHD:    100,
		},
		{
			Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
			InvoiceNumber:  "INV-DRFT-001",
			InvoiceDate:    invoiceDate,
			DueDate:        dueDate,
			CustomerID:     customerID,
			CustomerName:   "Finance Test Customer",
			Status:         "Draft",
			GrandTotalBHD:  50,
			OutstandingBHD: 50,
			SubtotalBHD:    50,
		},
		{
			Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
			InvoiceNumber:  "INV-PRO-001",
			InvoiceDate:    invoiceDate,
			DueDate:        dueDate,
			CustomerID:     customerID,
			CustomerName:   "Finance Test Customer",
			Status:         "Proforma",
			GrandTotalBHD:  25,
			OutstandingBHD: 25,
			SubtotalBHD:    25,
		},
	}
	require.NoError(t, app.db.Create(&invoices).Error)

	dashboard, err := app.GetDynamicFinancialDashboard(year)
	require.NoError(t, err)
	require.Equal(t, 100.0, dashboard.TradeReceivables)
	require.Equal(t, 100.0, dashboard.ARCurrent+dashboard.AR30_60+dashboard.AR60_90+dashboard.AROver90)
}

func TestGetDynamicFinancialDashboardDoesNotUseOfferPipelineAsRevenue(t *testing.T) {
	app := setupTestApp(t)

	offerYear := 2028
	require.NoError(t, app.db.Create(&Offer{
		Base:           Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:    "OFF-2028-001",
		QuotationDate:  time.Date(offerYear, time.January, 20, 0, 0, 0, 0, time.UTC),
		ValidityDate:   time.Date(offerYear, time.February, 20, 0, 0, 0, 0, time.UTC),
		CustomerID:     uuid.New().String(),
		CustomerName:   "Pipeline Only Customer",
		TotalValueBHD:  8750,
		Stage:          "Quoted",
		RevisionNumber: 1,
		Division:       "PH Trading",
	}).Error)

	dashboard, err := app.GetDynamicFinancialDashboard(offerYear)
	require.NoError(t, err)
	require.Zero(t, dashboard.Revenue)
	require.Zero(t, dashboard.GrossProfit)
	require.Zero(t, dashboard.TradeReceivables)
}
