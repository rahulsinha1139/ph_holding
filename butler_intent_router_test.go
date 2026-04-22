package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestButlerIntentClarification_CapabilitiesQuestionReturnsCommandChoices(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Conversation{}, &ChatMessage{}))

	resp, err := app.ChatWithButlerPersistent("", "hello buddy! I would love it if you could let me understand your capabilities")
	require.NoError(t, err)
	require.Contains(t, resp.Response, "Pick the view")
	require.NotEmpty(t, resp.Actions)
	require.Equal(t, "clarify", resp.Actions[0].Type)
	require.Equal(t, "butler_prompt", resp.Actions[0].Target)

	data, ok := resp.Actions[0].Data.(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, data["prompt"], "capabilities")

	var assistant ChatMessage
	require.NoError(t, app.db.Where("conversation_id = ? AND role = ?", resp.ConversationID, "assistant").First(&assistant).Error)
	require.Equal(t, "assistant_actionable", assistant.MessageType)
	require.Equal(t, "clarify", assistant.ActionType)
	require.Equal(t, "butler_prompt", assistant.ActionTarget)
	require.Contains(t, assistant.ActionMetadata, "butler_prompt")
}

func TestButlerIntentClarification_ARProjectionAsksForBasis(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Conversation{}, &ChatMessage{}))

	resp, err := app.ChatWithButlerPersistent("", "Yes Sir please get me the AR projections for the next month please!")
	require.NoError(t, err)
	require.Contains(t, resp.Response, "choice matters")
	require.Len(t, resp.Actions, 4)
	require.Equal(t, "Invoices only", resp.Actions[0].Label)
	require.Equal(t, "Include confirmed orders", resp.Actions[1].Label)
	require.Equal(t, "Add weighted offers", resp.Actions[2].Label)
}

func TestGroundedARProjection_IncludesConfirmedUninvoicedOrders(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Conversation{}, &ChatMessage{}))

	customerID := seedTestCustomer(t, app.db, "BAPCO")
	now := time.Now()
	nextMonthStart, _, _ := nextCalendarMonthWindow(now)
	orderDate := time.Date(now.Year(), time.January, 15, 0, 0, 0, 0, time.Local)
	if orderDate.After(now) {
		orderDate = now.AddDate(0, 0, -1)
	}

	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		OrderNumber:   "ORD-AR-001",
		CustomerID:    customerID,
		CustomerName:  "BAPCO",
		OrderDate:     orderDate,
		Status:        "Processing",
		GrandTotalBHD: 1000,
	}
	require.NoError(t, app.db.Create(&order).Error)

	invoice := Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		InvoiceNumber:  "INV-AR-001",
		CustomerID:     customerID,
		CustomerName:   "BAPCO",
		InvoiceDate:    now,
		DueDate:        nextMonthStart.AddDate(0, 0, 5),
		Status:         "Sent",
		GrandTotalBHD:  250,
		OutstandingBHD: 250,
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	resp, err := app.ChatWithButlerPersistent("", "but we have 21 orders in the pipeline! will we not have any receivable from them?")
	require.NoError(t, err)
	require.Contains(t, resp.Response, "Confirmed order receivable path")
	require.Contains(t, resp.Response, "1 orders")
	require.Contains(t, resp.Response, "1000.000 BHD")
	require.Contains(t, resp.Response, "Orders are now explicitly counted")
	require.NotEmpty(t, resp.Actions)
	require.Equal(t, "clarify", resp.Actions[0].Type)
}
