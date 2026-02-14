package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// XML Structs matching ehOnline-Shop export
type Basket struct {
	XMLName xml.Name `xml:"basket"`
	Header  Header   `xml:"header"`
	Items   []Item   `xml:"item"`
}

type Header struct {
	Date     string   `xml:"date"`
	Customer Customer `xml:"customer"`
}

type Customer struct {
	Name    string    `xml:"name"`
	Contact Contact   `xml:"contact"`
	Address []Address `xml:"address"`
}

type Contact struct {
	FirstName string `xml:"firstname"`
	LastName  string `xml:"lastname"`
	Email     string `xml:"connectivity>email"`
	Phone     string `xml:"connectivity>phone"`
}

type Address struct {
	Type    string `xml:"type,attr"`
	Name1   string `xml:"name1"`
	Street  string `xml:"street"`
	City    string `xml:"city"`
	Country string `xml:"country"`
}

type Item struct {
	Product Product `xml:"product"`
	Pricing Pricing `xml:"itemPricing"`
}

type Product struct {
	MaterialNo       string `xml:"materialNo"`
	OrderCode        string `xml:"orderCode"`
	ShortDescription string `xml:"texts>shortDescription"`
	LongDescription  string `xml:"texts>longDescription"`
}

type Pricing struct {
	UnitSalesPrice float64 `xml:"unitSalesPrice"` // Cost to PH
	Currency       string  `xml:"unitSalesPrice,attr"`
}

// Output Structs
type ExtractedData struct {
	Customers []ExtractedCustomer `json:"customers"`
	Products  []ExtractedProduct  `json:"products"`
}

type ExtractedCustomer struct {
	Name    string `json:"name"`
	Contact string `json:"contact"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type ExtractedProduct struct {
	Code        string  `json:"code"`
	Description string  `json:"description"`
	CostEUR     float64 `json:"cost_eur"`
}

func main() {
	rootPath := `C:\Users\rishu\OneDrive\Desktop\asymm\production_shenanigans\asymmetrica-production\ph_holdings_sovereign_ui\Offers No 1-50 (2025)`
	
data := ExtractedData{
		Customers: make([]ExtractedCustomer, 0),
		Products:  make([]ExtractedProduct, 0),
	}

	customerMap := make(map[string]bool)
	productMap := make(map[string]bool)

	fmt.Printf("🔍 Scanning artifacts in: %s\n", rootPath)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".xml") {
			fmt.Printf("  📄 Parsing: %s\n", info.Name())
			processXML(path, &data, customerMap, productMap)
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	// Output results
	file, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile("seed_data.json", file, 0644)

	fmt.Printf("\n✅ Extraction Complete!\n")
	fmt.Printf("   Customers Found: %d\n", len(data.Customers))
	fmt.Printf("   Products Found:  %d\n", len(data.Products))
	fmt.Println("   Saved to: seed_data.json")
}

func processXML(path string, data *ExtractedData, custMap, prodMap map[string]bool) {
	xmlFile, err := os.Open(path)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer xmlFile.Close()

	byteValue, _ := io.ReadAll(xmlFile)

	var basket Basket
	if err := xml.Unmarshal(byteValue, &basket); err != nil {
		// Try ignoring namespace if direct unmarshal fails (common in complex XMLs)
		// For now, let's assume standard format works or simple failure
		fmt.Println("XML Parse Error:", err)
		return
	}

	// Extract Customer
	custName := basket.Header.Customer.Name
	if custName != "" && !custMap[custName] {
		// Find main address
		var addr Address
		for _, a := range basket.Header.Customer.Address {
			if a.Type == "SOLD_TO" {
				addr = a
				break
			}
		}

		data.Customers = append(data.Customers, ExtractedCustomer{
			Name:    custName,
			Contact: fmt.Sprintf("%s %s", basket.Header.Customer.Contact.FirstName, basket.Header.Customer.Contact.LastName),
			Email:   basket.Header.Customer.Contact.Email,
			Phone:   basket.Header.Customer.Contact.Phone,
			Address: addr.Street,
			City:    addr.City,
			Country: addr.Country,
		})
		custMap[custName] = true
	}

	// Extract Products
	for _, item := range basket.Items {
		code := item.Product.OrderCode
		if code != "" && !prodMap[code] {
			data.Products = append(data.Products, ExtractedProduct{
				Code:        code,
				Description: item.Product.ShortDescription,
				CostEUR:     item.Pricing.UnitSalesPrice,
			})
			prodMap[code] = true
		}
	}
}
