# PH Holdings Sovereign UI

Desktop ERP application for **PH Trading WLL** (Bahrain) built with Go + Svelte + Wails.

## Tech Stack

- **Backend**: Go 1.24 + Wails v2.11
- **Frontend**: Svelte + TypeScript + Vite
- **Database**: SQLite (primary) + Supabase PostgreSQL (cloud sync)
- **AI**: Mistral API (Butler intelligence assistant)
- **PDF**: gofpdf + gopdf (invoices, offers, PO, reports on company letterhead)
- **Platforms**: Windows (primary), macOS (Apple Silicon supported)

## Quick Start

### Prerequisites

- Go 1.24+
- Node.js 20+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Development

```bash
# Install dependencies
go mod download
cd frontend && npm install && cd ..

# Copy and edit environment config
cp .env.example .env
# Edit .env with your database URL and API keys

# Run in dev mode (hot reload)
wails dev

# Production build
wails build -clean
```

### macOS Setup

See `mac_dev_setup/MAC_SETUP_CHECKLIST.md` for comprehensive Apple Silicon setup guide.

## Project Structure

```
ph_holdings_sovereign_ui/
  app.go                    # Main backend (11,000+ LOC)
  main.go                   # Entry point
  database.go               # GORM models and schema
  butler_ai.go              # Mistral AI chat engine
  butler_reports.go         # AI-powered PDF reports
  config.go                 # Configuration management
  *_service.go              # Domain services (bank, invoice, payment, etc.)
  *_pdf_service.go          # PDF generation (offer, invoice, PO)
  frontend/
    src/
      App.svelte            # Main router
      lib/
        screens/            # Page components (20+ screens)
        components/         # Reusable UI components
        stores/             # Svelte stores
    wailsjs/                # Auto-generated Go bindings
  cmd/                      # CLI utilities (import, sync, migrate)
  pkg/                      # Go packages (OCR, graph, engines)
  integration/              # Microsoft Graph + COM automation
  CRITICAL_PH_CONTEXT_SSOT/ # Company templates and letterhead
  wails.json                # Wails configuration
```

## Features

- **Sales Pipeline**: RFQ -> Costing -> Offer -> Order -> Invoice -> Payment
- **CRM Hub**: Customer/Supplier dashboards, 360 profiles, notes, issue tracking
- **Finance Hub**: Financial dashboard (audited FS2024 data), AR aging, payments
- **Operations Hub**: Purchase Orders, Delivery Notes, GRN with partial receiving
- **Butler AI**: Natural language queries, PDF report generation on letterhead
- **Bank Reconciliation**: Statement import, transaction matching, account management
- **RBAC**: License-based activation with 5 roles (Admin, Manager, Sales, Operations, Staff)
- **Cloud Sync**: SQLite primary + Supabase background sync (works offline)
- **Document OCR**: AI-powered document classification and data extraction

## Environment Variables

```bash
# Database (Render.com PostgreSQL)
DATABASE_URL=postgresql://user:pass@host:5432/dbname

# AI (Mistral)
MISTRAL_API_KEY=your-key

# Optional
ENABLE_FILE_WATCHER=true
LOG_LEVEL=info
```

## Testing

```bash
go test ./...              # Go tests
cd frontend && npm run check  # TypeScript check
```

## Business Context

**PH Trading WLL** - Bahrain-based process instrumentation company.
Suppliers: Endress+Hauser, Servomex, Landis+Gyr.
Customers: BAPCO, Alba, EWA. Currency: BHD (3 decimal places). VAT: 10%.

## License

Proprietary - All Rights Reserved
