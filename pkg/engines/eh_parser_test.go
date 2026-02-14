package engines

import (
	"testing"
)

func TestEHParser_ParseBatch_Errors(t *testing.T) {
	parser := NewEHParser()
	
	// Test with non-existent directory
	_, err := parser.ParseBatch("non_existent_dir_12345")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestEHParser_ClassifyProductType(t *testing.T) {
	parser := NewEHParser()
	
	tests := []struct {
		orderCode   string
		description string
		want        string
	}{
		{"CM442-3RT0/0", "Liquiline CM442 transmitter", "E+H Flow"},
		{"FMU90-R11CA111AA3A", "Prosonic FMU90 level measurement", "E+H Level"},
		{"53P50-EA0A1AA0ACAA", "Cerabar PMC51 pressure sensor", "E+H Pressure"},
		{"TMT162-A1BA2ADAA1", "iTHERM TrustSens temperature", "E+H Temperature"},
		{"10W40-UA0A1AA0ACAA", "Promag W 400 flowmeter", "E+H Flow"},
		{"UNKNOWN", "Unknown device", "E+H General"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			if got := parser.ClassifyProductType(tt.orderCode, tt.description); got != tt.want {
				t.Errorf("EHParser.ClassifyProductType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEHParser_Conversion(t *testing.T) {
	// 100 EUR * 0.41 = 41 BHD
	valEUR := 100.0
	expectedBHD := 41.0
	
	got := valEUR * EUR_TO_BHD
	if got != expectedBHD {
		t.Errorf("Currency conversion failed: 100 EUR = %v BHD, want %v", got, expectedBHD)
	}
}
