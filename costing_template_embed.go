package main

import (
	"bytes"
	"fmt"

	_ "embed"

	"github.com/xuri/excelize/v2"
)

//go:embed "templates/PH Trading Costing MasterFile.xlsx"
var embeddedCostingTemplate []byte

func openCostingTemplateWorkbook(projectRoot string) (*excelize.File, string, error) {
	if templatePath := findCostingTemplatePath(projectRoot); templatePath != "" {
		workbook, err := excelize.OpenFile(templatePath)
		if err != nil {
			return nil, "", fmt.Errorf("open filesystem template %q: %w", templatePath, err)
		}
		return workbook, templatePath, nil
	}

	if len(embeddedCostingTemplate) == 0 {
		return nil, "", fmt.Errorf("embedded PH costing template is empty")
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(embeddedCostingTemplate))
	if err != nil {
		return nil, "", fmt.Errorf("open embedded PH costing template: %w", err)
	}
	return workbook, "embedded:templates/PH Trading Costing MasterFile.xlsx", nil
}
