package engines

import (
	"testing"
)

func TestCostingEngine_GetProductMargin(t *testing.T) {
	ce := NewCostingEngine()
	
	tests := []struct {
		productType string
		want        float64
	}{
		{"E+H Flow", 0.15},
		{"E+H Level", 0.18},
		{"E+H Pressure", 0.18},
		{"E+H Temperature", 0.15},
		{"E+H Analytics", 0.20},
		{"E+H General", 0.12},
		{"Servomex", 0.25},
		{"GIC", 0.10},
		{"Unknown", 0.12}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.productType, func(t *testing.T) {
			if got := ce.GetProductMargin(tt.productType); got != tt.want {
				t.Errorf("CostingEngine.GetProductMargin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCostingEngine_GetCustomerDiscount(t *testing.T) {
	ce := NewCostingEngine()
	
	tests := []struct {
		grade CustomerGrade
		want  float64
	}{
		{GradeA, 0.07},
		{GradeB, 0.03},
		{GradeC, 0.00},
		{GradeD, 0.00},
		{"Unknown", 0.00},
	}

	for _, tt := range tests {
		t.Run(string(tt.grade), func(t *testing.T) {
			if got := ce.GetCustomerDiscount(tt.grade); got != tt.want {
				t.Errorf("CostingEngine.GetCustomerDiscount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCostingEngine_GenerateCostingFromBasket(t *testing.T) {
	ce := NewCostingEngine()
	
	basket := &ParsedEHBasket{
		SourceFile: "test_basket.xml",
		Items: []ParsedEHItem{
			{
				OrderCode:         "CM442",
				Description:       "Liquiline",
				Quantity:          2,
				ProductType:       "E+H Flow",
				ItemSalesPriceBHD: 1000.0, // Total cost for 2 items
				ProductionDays:    3,
			},
		},
	}

	sheet := ce.GenerateCostingFromBasket(basket, "BAPCO", GradeA)
	
	if sheet.CustomerName != "BAPCO" {
		t.Errorf("Expected customer BAPCO, got %s", sheet.CustomerName)
	}
	
	if sheet.CustomerGrade != GradeA {
		t.Errorf("Expected Grade A, got %s", sheet.CustomerGrade)
	}

	if len(sheet.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(sheet.Items))
	}

	item := sheet.Items[0]
	// Cost = 1000.0 / 2 = 500.0 per unit
	// Margin for Flow = 15% -> Sell Price = Cost / (1 - 0.15) = 500 / 0.85 = 588.235
	// Discount for Grade A = 7%
	
	if item.ProductType != "E+H Flow" {
		t.Errorf("Expected E+H Flow, got %s", item.ProductType)
	}

	if item.TotalCostBHD != 1000.0 {
		t.Errorf("Expected Total Cost 1000.0, got %v", item.TotalCostBHD)
	}
}
