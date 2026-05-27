package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCRMCustomerDashboardLinksMappedCustomerNamesAndOrderExposure(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerNameMapping{}))

	customer := CustomerMaster{
		Base:         Base{ID: uuid.New().String()},
		CustomerID:   "C-ACME",
		CustomerCode: "ACME-001",
		BusinessName: "Acme Industrial W.L.L",
		PaymentGrade: "B",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)
	require.NoError(t, app.db.Create(&CustomerNameMapping{
		ID:            uuid.New().String(),
		ExtractedName: "ACME Site Office",
		CanonicalName: customer.BusinessName,
		CustomerID:    customer.ID,
		Verified:      true,
	}).Error)

	orderDate := time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC)
	order := Order{
		Base:          Base{ID: uuid.New().String()},
		OrderNumber:   "ORD-LINK-001",
		CustomerName:  "ACME Site Office",
		OrderDate:     orderDate,
		RequiredDate:  orderDate.AddDate(0, 0, 30),
		GrandTotalBHD: 1000,
		Status:        "Confirmed",
	}
	require.NoError(t, app.db.Create(&order).Error)

	draftInvoice := Invoice{
		Base:           Base{ID: uuid.New().String()},
		InvoiceNumber:  "DRAFT-LINK-001",
		InvoiceDate:    orderDate,
		CustomerID:     customer.ID,
		CustomerName:   customer.BusinessName,
		OrderID:        order.ID,
		GrandTotalBHD:  250,
		OutstandingBHD: 250,
		Status:         "Draft",
		DueDate:        orderDate.AddDate(0, 0, 30),
	}
	require.NoError(t, app.db.Create(&draftInvoice).Error)

	dashboard := app.GetCRMCustomerDashboardByYear(2026)

	require.Equal(t, 1, dashboard.ActiveCustomers)
	require.Equal(t, 1000.0, dashboard.TotalRevenue)
	require.Equal(t, 1000.0, dashboard.TotalOutstanding)
	require.Len(t, dashboard.TopCustomers, 1)
	require.Equal(t, customer.ID, dashboard.TopCustomers[0].ID)
	require.Equal(t, 1000.0, dashboard.TopCustomers[0].TotalRevenue)
	require.Equal(t, 1000.0, dashboard.TopCustomers[0].OutstandingBHD)
}

func TestCustomerFullProfileLinksOrdersOffersInvoicesAndPayments(t *testing.T) {
	app := setupTestApp(t)

	customer := CustomerMaster{
		Base:         Base{ID: uuid.New().String()},
		CustomerID:   "C-ALDUR",
		CustomerCode: "ALDUR-001",
		BusinessName: "Aldur",
		PaymentGrade: "C",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	orderDate := time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC)
	order := Order{
		Base:          Base{ID: uuid.New().String()},
		OrderNumber:   "ORD-LINK-002",
		CustomerName:  "Aldur WLL",
		OrderDate:     orderDate,
		GrandTotalBHD: 2000,
		Status:        "Processing",
	}
	require.NoError(t, app.db.Create(&order).Error)

	offer := Offer{
		Base:          Base{ID: uuid.New().String()},
		OfferNumber:   "OFF-LINK-001",
		CustomerName:  "ALDUR W.L.L",
		QuotationDate: orderDate.AddDate(0, 0, -7),
		TotalValueBHD: 500,
		Stage:         "Quoted",
	}
	require.NoError(t, app.db.Create(&offer).Error)

	invoice := Invoice{
		Base:           Base{ID: uuid.New().String()},
		InvoiceNumber:  "INV-LINK-001",
		InvoiceDate:    orderDate.AddDate(0, 0, 5),
		CustomerID:     customer.CustomerCode,
		GrandTotalBHD:  300,
		OutstandingBHD: 300,
		Status:         "Sent",
		DueDate:        orderDate.AddDate(0, 0, 35),
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	payment := Payment{
		Base:          Base{ID: uuid.New().String()},
		InvoiceID:     invoice.ID,
		InvoiceNumber: invoice.InvoiceNumber,
		AmountBHD:     120,
		PaymentDate:   orderDate.AddDate(0, 0, 20),
		PaymentMethod: "Bank Transfer",
	}
	require.NoError(t, app.db.Create(&payment).Error)

	profile, err := app.GetCustomerFullProfile(customer.ID)
	require.NoError(t, err)

	require.Equal(t, 1, profile.TotalOrders)
	require.Equal(t, 2000.0, profile.AvgOrderValue)
	require.Equal(t, 2800.0, profile.TotalRevenue)
	require.Equal(t, 300.0, profile.OutstandingBHD)
	require.GreaterOrEqual(t, profile.RFQsFloated, 1)
	require.Len(t, profile.RecentOrders, 1)
	require.Len(t, profile.RecentInvoices, 1)
	require.Len(t, profile.PaymentHistory, 1)
	require.Equal(t, invoice.InvoiceNumber, profile.PaymentHistory[0].InvoiceNumber)
}

func TestCustomerFullProfileDedupesLinkedOfferAndOpportunityRows(t *testing.T) {
	app := setupTestApp(t)

	customer := CustomerMaster{
		Base:         Base{ID: uuid.New().String()},
		CustomerID:   "C-BAPCO",
		CustomerCode: "BAPCO-001",
		BusinessName: "The Bahrain Petroleum Co. (Bapco)",
		PaymentGrade: "C",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	offerDate := time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)
	offer := Offer{
		Base:          Base{ID: uuid.New().String()},
		OfferNumber:   "OTH-07-26",
		CustomerID:    customer.ID,
		CustomerName:  customer.BusinessName,
		QuotationDate: offerDate,
		TotalValueBHD: 72464.04,
		Stage:         "Quoted",
	}
	require.NoError(t, app.db.Create(&offer).Error)

	require.NoError(t, app.db.Create(&Opportunity{
		Base:         Base{ID: uuid.New().String()},
		FolderNumber: "2026-307",
		OfferID:      offer.ID,
		CustomerID:   customer.ID,
		CustomerName: customer.BusinessName,
		Year:         2026,
		Title:        "BAPCO UPSTREAM MSA",
		OfferDate:    offerDate,
		RevenueBHD:   72464.04,
		Stage:        "Quoted",
	}).Error)
	require.NoError(t, app.db.Create(&Opportunity{
		Base:         Base{ID: uuid.New().String()},
		FolderNumber: "OTH-07-26",
		OfferID:      offer.ID,
		CustomerID:   customer.ID,
		CustomerName: customer.BusinessName,
		Year:         2026,
		Title:        "BAPCO UPSTREAM MSA",
		OfferDate:    offerDate,
		RevenueBHD:   72464.04,
		Stage:        "Quoted",
	}).Error)

	profile, err := app.GetCustomerFullProfile(customer.ID)
	require.NoError(t, err)

	require.Equal(t, 1, profile.RFQsFloated)
	require.Len(t, profile.RecentRFQs, 1)
	require.Equal(t, "BAPCO UPSTREAM MSA", profile.RecentRFQs[0].Project)
	require.Equal(t, 72464.04, profile.TotalRevenue)
}
