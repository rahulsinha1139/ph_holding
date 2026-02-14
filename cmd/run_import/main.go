package main

import (
	"log"
	"time"

	"ph_holdings_app/pkg/data"
	"ph_holdings_app/pkg/graph"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// This matches the models in app.go to ensure migration
type PredictionRecord struct {
	ID            uint      `gorm:"primaryKey"`
	CustomerID    string    `gorm:"index"`
	CustomerName  string    `gorm:"index"`
	Grade         string    `gorm:"size:5"`
	PredictedDays float64   `json:"predicted_days"`
	Confidence    float64   `json:"confidence"`
	R1            float64   `json:"r1"`
	R2            float64   `json:"r2"`
	R3            float64   `json:"r3"`
	CreatedAt     time.Time `gorm:"index"`
}

func main() {
	dbPath := "ph_holdings.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	log.Println("🚀 Starting Operational Readiness Run...")

	// 1. Migrate all necessary tables
	log.Println("📦 Migrating tables...")
	err = db.AutoMigrate(
		&PredictionRecord{},
		&data.CustomerMaster{},
		&data.OpportunitySSOT{},
		&data.PaymentSSOT{},
		&data.ProductCostingSSOT{},
		&graph.GraphNode{},
		&graph.GraphEdge{},
	)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// 2. Import SSOT Data
	dataDir := "CRITICAL_PH_CONTEXT_SSOT"
	log.Printf("📥 Importing SSOT data from %s...", dataDir)
	
	importResult, err := data.ImportAllSSOT(db, dataDir)
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}

	log.Printf("✅ Import Success!")
	log.Printf("   - Customers: %d", importResult.CustomersImported)
	log.Printf("   - Opportunities: %d", importResult.OpportunitiesImported)
	log.Printf("   - Payments: %d", importResult.PaymentsImported)
	log.Printf("   - Products: %d", importResult.ProductsImported)
	log.Printf("   - Duration: %v", importResult.Duration)

	// 3. Build Entity Graph
	log.Println("🕸️ Building Entity Graph...")
	builder := graph.NewSSOTBuilder(db)
	graphStats, err := builder.BuildGraph()
	if err != nil {
		log.Fatalf("graph build failed: %v", err)
	}

	log.Printf("✅ Graph Build Success!")
	log.Printf("   - Nodes Created: %d", graphStats.NodesCreated)
	log.Printf("   - Edges Created: %d", graphStats.EdgesCreated)

	log.Println("✨ Operational Readiness Run Complete!")
}
