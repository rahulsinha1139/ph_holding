package main

import (
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGenerateDeliveryNotePDFKeepsFiveCompactRowsOnOnePage(t *testing.T) {
	t.Chdir(t.TempDir())
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&DeliveryNote{}, &DeliveryNoteItem{}))

	now := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	customerID := seedTestCustomer(t, app.db, "AL EZZEL OPERATION AND MAINTENANCE COMPANY W.L.L")
	orderID := uuid.New().String()

	order := Order{
		Base:             Base{ID: orderID, CreatedAt: now, UpdatedAt: now},
		OrderNumber:      "ORD-PH25/001",
		CustomerPONumber: "PH25/001",
		CustomerID:       customerID,
		CustomerName:     "AL EZZEL OPERATION AND MAINTENANCE COMPANY W.L.L",
		OrderDate:        time.Date(2025, 12, 30, 0, 0, 0, 0, time.UTC),
		RequiredDate:     now,
		Status:           "Delivered",
		Division:         "PH Trading",
	}
	require.NoError(t, app.db.Create(&order).Error)

	items := []DeliveryNoteItem{
		{Description: "Per Order ORD-PH25/001"},
		{Description: "Promag P 300, 5P3B1H, DN100 4\" - 5P3B1H-24897/101"},
		{Description: "Promag, grounding disc/protection disc - DK5GD-14N3/0"},
		{Description: "Promag P 300, 5P3B1F, DN150 6\" - 5P3B1F-1NDD9/101"},
		{Description: "Promag, grounding disc/protection disc - DK5GD-11J4/0"},
	}
	for i := range items {
		items[i].Base = Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now}
		items[i].QuantityOrdered = 1
		items[i].QuantityDelivered = 1
	}

	dn := DeliveryNote{
		Base:            Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		DNNumber:        "DN-ORD-PH25/001",
		OrderID:         orderID,
		CustomerID:      customerID,
		DeliveryDate:    now,
		DeliveryAddress: "AL EZZEL OPERATION AND MAINTENANCE COMPANY W.L.L\nManama",
		Status:          "Delivered",
		Items:           items,
	}
	require.NoError(t, app.db.Create(&dn).Error)

	path, err := app.GenerateDeliveryNotePDF(dn.ID)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	pageMarkers := regexp.MustCompile(`/Type\s*/Page\b`).FindAll(data, -1)
	require.Len(t, pageMarkers, 1)
}
