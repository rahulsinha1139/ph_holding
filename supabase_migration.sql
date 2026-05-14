-- ═══════════════════════════════════════════════════════════════════════════
-- SUPABASE MANUAL MIGRATION SCRIPT
-- AsymmFlow ERP — Phase 34 (Butler Pipeline Rebuild)
-- Run against: postgres.xkyydcxxkzjbnsrmzkpp @ aws-1-ap-southeast-1.pooler.supabase.com
--
-- INSTRUCTIONS:
--   PGPASSWORD='<supabase-db-password>' /opt/homebrew/opt/libpq/bin/psql \
--     -h aws-1-ap-southeast-1.pooler.supabase.com -p 5432 \
--     -U postgres.xkyydcxxkzjbnsrmzkpp -d postgres \
--     -f supabase_migration.sql
--
-- This session added NO new database columns. All changes were to:
--   - Go structs (ButlerOfferDraftRequest.CustomerID — request DTO, not a table)
--   - AIML model IDs (Go constants, not in DB)
--   - Export paths (filesystem, not DB)
--   - Butler system prompt (Go code, not DB)
--   - Frontend Svelte code
--
-- However, the following tables in dbSyncTables should be verified to exist
-- on Supabase with correct schema. Run this script to ensure readiness.
-- ═══════════════════════════════════════════════════════════════════════════

-- Verify key tables exist (these were added in Phase 33 but Supabase was down)
CREATE TABLE IF NOT EXISTS opportunities (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    year INTEGER DEFAULT 0,
    opp_number TEXT DEFAULT '',
    folder_name TEXT DEFAULT '',
    title TEXT DEFAULT '',
    customer_id TEXT DEFAULT '',
    customer_name TEXT DEFAULT '',
    stage TEXT DEFAULT '',
    eh_ref TEXT DEFAULT '',
    comment TEXT DEFAULT '',
    owner_notes TEXT DEFAULT '',
    source TEXT DEFAULT '',
    payment_terms TEXT DEFAULT '',
    delivery_terms TEXT DEFAULT '',
    value_bhd DOUBLE PRECISION DEFAULT 0,
    probability INTEGER DEFAULT 0,
    expected_close_date TIMESTAMP WITH TIME ZONE,
    actual_close_date TIMESTAMP WITH TIME ZONE,
    win_reason TEXT DEFAULT '',
    loss_reason TEXT DEFAULT '',
    competitor TEXT DEFAULT '',
    product_category TEXT DEFAULT '',
    contact_person TEXT DEFAULT '',
    division TEXT DEFAULT '',
    priority TEXT DEFAULT '',
    next_action TEXT DEFAULT '',
    next_action_date TIMESTAMP WITH TIME ZONE,
    rfq_id TEXT DEFAULT '',
    offer_id TEXT DEFAULT '',
    order_id TEXT DEFAULT '',
    invoice_id TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    custom_field_1 TEXT DEFAULT '',
    custom_field_2 TEXT DEFAULT '',
    custom_field_3 TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS rfq_data (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS costing_sheet_data (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS costing_line_items (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS costing_history (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS offer_data (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT ''
);

-- Phase 32 columns (verify they exist)
ALTER TABLE offers ADD COLUMN IF NOT EXISTS quote_type VARCHAR(50) DEFAULT 'Quotation';
ALTER TABLE offers ADD COLUMN IF NOT EXISTS vat_rate DOUBLE PRECISION DEFAULT 10;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS mobile_number VARCHAR(50) DEFAULT '';

-- Phase 34: Customer enrichment fields
ALTER TABLE customers ADD COLUMN IF NOT EXISTS trading_name VARCHAR(255) DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS cr_number VARCHAR(100) DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'Active';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS primary_phone VARCHAR(50) DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS primary_email VARCHAR(255) DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS website VARCHAR(255) DEFAULT '';

-- Verify conversations and chat_messages tables exist for Butler persistence
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    title VARCHAR(255) DEFAULT '',
    summary VARCHAR(2000) DEFAULT '',
    is_active BOOLEAN DEFAULT TRUE,
    last_msg_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    role VARCHAR(20) DEFAULT '',
    content TEXT DEFAULT '',
    tokens_used INTEGER DEFAULT 0,
    message_type VARCHAR(50) DEFAULT 'chat',
    action_type VARCHAR(50) DEFAULT '',
    action_target VARCHAR(100) DEFAULT '',
    action_label VARCHAR(100) DEFAULT '',
    action_data TEXT DEFAULT '',
    action_status VARCHAR(50) DEFAULT 'none',
    action_metadata TEXT DEFAULT ''
);

-- Phase 37: Customer receipt headers and on-account allocations.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS receipt_id TEXT;

CREATE TABLE IF NOT EXISTS customer_receipts (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    receipt_number VARCHAR(50) UNIQUE,
    customer_id TEXT DEFAULT '',
    customer_name VARCHAR(255) DEFAULT '',
    division VARCHAR(100) DEFAULT 'PH Trading',
    receipt_date TIMESTAMP WITH TIME ZONE,
    amount_bhd DOUBLE PRECISION DEFAULT 0,
    applied_amount_bhd DOUBLE PRECISION DEFAULT 0,
    unapplied_amount_bhd DOUBLE PRECISION DEFAULT 0,
    payment_method VARCHAR(50) DEFAULT '',
    reference VARCHAR(100) DEFAULT '',
    status VARCHAR(30) DEFAULT 'OnAccount',
    notes TEXT DEFAULT '',
    updated_by TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS customer_receipt_allocations (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    receipt_id TEXT DEFAULT '',
    invoice_id TEXT DEFAULT '',
    invoice_number VARCHAR(50) DEFAULT '',
    payment_id TEXT DEFAULT '',
    allocated_amount_bhd DOUBLE PRECISION DEFAULT 0,
    allocation_date TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) DEFAULT 'Applied',
    updated_by TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_customer_receipts_customer
    ON customer_receipts(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_receipts_division
    ON customer_receipts(division);
CREATE INDEX IF NOT EXISTS idx_customer_receipt_allocations_receipt
    ON customer_receipt_allocations(receipt_id);
CREATE INDEX IF NOT EXISTS idx_customer_receipt_allocations_invoice
    ON customer_receipt_allocations(invoice_id);

-- Phase 38: Costing technical datasheets.
CREATE TABLE IF NOT EXISTS costing_sheet_attachments (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    scope_id VARCHAR(160) DEFAULT '',
    costing_number VARCHAR(120) DEFAULT '',
    file_name VARCHAR(255) DEFAULT '',
    file_ext VARCHAR(20) DEFAULT '',
    mime_type VARCHAR(100) DEFAULT '',
    file_size BIGINT DEFAULT 0,
    file_hash VARCHAR(64) DEFAULT '',
    storage_mode VARCHAR(30) DEFAULT 'database',
    local_path VARCHAR(1000) DEFAULT '',
    content_base64 TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    uploaded_by VARCHAR(120) DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_costing_sheet_attachments_scope
    ON costing_sheet_attachments(scope_id);
CREATE INDEX IF NOT EXISTS idx_costing_sheet_attachments_costing
    ON costing_sheet_attachments(costing_number);
CREATE INDEX IF NOT EXISTS idx_costing_sheet_attachments_hash
    ON costing_sheet_attachments(file_hash);

-- Phase 38: User activity monitoring sync tables.
CREATE TABLE IF NOT EXISTS user_activity_sessions (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    session_id VARCHAR(64) UNIQUE,
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    user_id VARCHAR(120) DEFAULT '',
    employee_id VARCHAR(36) DEFAULT '',
    employee_name VARCHAR(120) DEFAULT '',
    license_key_hash VARCHAR(64) DEFAULT '',
    license_role VARCHAR(40) DEFAULT '',
    device_hash VARCHAR(64) DEFAULT '',
    source VARCHAR(40) DEFAULT '',
    primary_screen VARCHAR(100) DEFAULT '',
    is_open BOOLEAN DEFAULT TRUE,
    active_seconds INTEGER DEFAULT 0,
    meaningful_seconds INTEGER DEFAULT 0,
    idle_seconds INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,
    search_count INTEGER DEFAULT 0,
    create_count INTEGER DEFAULT 0,
    update_count INTEGER DEFAULT 0,
    export_count INTEGER DEFAULT 0,
    navigation_count INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_activity_events (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    event_time TIMESTAMP WITH TIME ZONE,
    event_type VARCHAR(40) DEFAULT '',
    category VARCHAR(40) DEFAULT '',
    screen VARCHAR(100) DEFAULT '',
    route VARCHAR(255) DEFAULT '',
    action_label VARCHAR(255) DEFAULT '',
    action_key VARCHAR(120) DEFAULT '',
    resource_type VARCHAR(80) DEFAULT '',
    resource_id VARCHAR(120) DEFAULT '',
    search_text VARCHAR(255) DEFAULT '',
    search_hash VARCHAR(64) DEFAULT '',
    search_redacted BOOLEAN DEFAULT FALSE,
    metadata_json TEXT DEFAULT '',
    user_id VARCHAR(120) DEFAULT '',
    employee_id VARCHAR(36) DEFAULT '',
    employee_name VARCHAR(120) DEFAULT '',
    license_key_hash VARCHAR(64) DEFAULT '',
    license_role VARCHAR(40) DEFAULT '',
    device_hash VARCHAR(64) DEFAULT '',
    session_id VARCHAR(64) DEFAULT '',
    active_seconds INTEGER DEFAULT 0,
    meaningful_seconds INTEGER DEFAULT 0,
    idle_seconds INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS user_activity_weekly_summaries (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    week_start TIMESTAMP WITH TIME ZONE,
    week_end TIMESTAMP WITH TIME ZONE,
    generated_at TIMESTAMP WITH TIME ZONE,
    user_key VARCHAR(120) DEFAULT '',
    user_id VARCHAR(120) DEFAULT '',
    employee_id VARCHAR(36) DEFAULT '',
    employee_name VARCHAR(120) DEFAULT '',
    license_key_hash VARCHAR(64) DEFAULT '',
    license_role VARCHAR(40) DEFAULT '',
    device_hash VARCHAR(64) DEFAULT '',
    active_seconds INTEGER DEFAULT 0,
    meaningful_seconds INTEGER DEFAULT 0,
    idle_seconds INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,
    search_count INTEGER DEFAULT 0,
    create_count INTEGER DEFAULT 0,
    update_count INTEGER DEFAULT 0,
    export_count INTEGER DEFAULT 0,
    navigation_count INTEGER DEFAULT 0,
    efficiency_score DOUBLE PRECISION DEFAULT 0,
    top_screens_json TEXT DEFAULT '',
    top_actions_json TEXT DEFAULT '',
    top_searches_json TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_user_activity_events_session
    ON user_activity_events(session_id);
CREATE INDEX IF NOT EXISTS idx_user_activity_events_employee
    ON user_activity_events(employee_id);
CREATE INDEX IF NOT EXISTS idx_user_activity_events_screen
    ON user_activity_events(screen);
CREATE INDEX IF NOT EXISTS idx_user_activity_sessions_employee
    ON user_activity_sessions(employee_id);
CREATE INDEX IF NOT EXISTS idx_user_activity_weekly_summaries_user
    ON user_activity_weekly_summaries(user_key);

-- Phase 36: Bank reconciliation many-to-many allocations.
-- Enables one bank receipt/payment to clear multiple invoices and one invoice
-- to be allocated across multiple bank statement lines.
CREATE TABLE IF NOT EXISTS bank_line_payment_allocations (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    bank_statement_line_id TEXT DEFAULT '',
    allocation_type VARCHAR(30) DEFAULT '',
    customer_invoice_id TEXT,
    customer_payment_id TEXT,
    customer_receipt_id TEXT,
    supplier_invoice_id TEXT,
    expense_entry_id TEXT,
    allocated_amount DOUBLE PRECISION DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'BHD',
    status VARCHAR(20) DEFAULT 'Allocated'
);

CREATE INDEX IF NOT EXISTS idx_bank_line_payment_allocations_line
    ON bank_line_payment_allocations(bank_statement_line_id);
CREATE INDEX IF NOT EXISTS idx_bank_line_payment_allocations_customer_invoice
    ON bank_line_payment_allocations(customer_invoice_id);
CREATE INDEX IF NOT EXISTS idx_bank_line_payment_allocations_customer_payment
    ON bank_line_payment_allocations(customer_payment_id);
CREATE INDEX IF NOT EXISTS idx_bank_line_payment_allocations_customer_receipt
    ON bank_line_payment_allocations(customer_receipt_id);
CREATE INDEX IF NOT EXISTS idx_bank_line_payment_allocations_supplier_invoice
    ON bank_line_payment_allocations(supplier_invoice_id);

ALTER TABLE bank_line_payment_allocations ADD COLUMN IF NOT EXISTS customer_payment_id TEXT;
ALTER TABLE bank_line_payment_allocations ADD COLUMN IF NOT EXISTS customer_receipt_id TEXT;

-- Phase 35: Sync schema drift repair for Sales OCR -> Opportunity -> Offer flow.
-- These ALTERs make the previously skeletal Supabase tables compatible with the
-- current Go structs used by dbSyncTables.
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS folder_number TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS customer_grade TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS salesperson TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS product_details TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS offer_date TIMESTAMP WITH TIME ZONE;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS order_date TIMESTAMP WITH TIME ZONE;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS expected_date TIMESTAMP WITH TIME ZONE;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS closed_date TIMESTAMP WITH TIME ZONE;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS revenue_bhd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS cost_bhd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS profit_bhd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS spoc_status TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS wip_status TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS regime INTEGER DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS r1 DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS r2 DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS r3 DOUBLE PRECISION DEFAULT 0;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS has_abb_competition BOOLEAN DEFAULT FALSE;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS product_type TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS won_reason TEXT DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS lost_reason TEXT DEFAULT '';
ALTER TABLE opportunities ALTER COLUMN opp_number TYPE INTEGER
    USING CASE WHEN opp_number::text ~ '^[0-9]+$' THEN opp_number::text::integer ELSE 0 END;

ALTER TABLE rfq_data ALTER COLUMN id TYPE BIGINT
    USING CASE WHEN id::text ~ '^[0-9]+$' THEN id::text::bigint ELSE 0 END;
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS rfq_number TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS client TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS project TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS value DOUBLE PRECISION DEFAULT 0;
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS notes TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'pending';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS stage TEXT DEFAULT 'RFQ Received';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS document_hash TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS visit_locations TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS product_details TEXT DEFAULT '';
ALTER TABLE rfq_data ADD COLUMN IF NOT EXISTS source_doc_path TEXT DEFAULT '';

ALTER TABLE costing_sheet_data ALTER COLUMN id TYPE BIGINT
    USING CASE WHEN id::text ~ '^[0-9]+$' THEN id::text::bigint ELSE 0 END;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS rfq_id BIGINT DEFAULT 0;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS rfq_name TEXT DEFAULT '';
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS revision_number INTEGER DEFAULT 1;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS parent_costing_id BIGINT;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS items TEXT DEFAULT '';
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS subtotal DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS total_markup DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS final_price DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS margin_percent DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'draft';
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS approved_by TEXT DEFAULT '';
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS approval_required BOOLEAN DEFAULT FALSE;
ALTER TABLE costing_sheet_data ADD COLUMN IF NOT EXISTS risk_warnings TEXT DEFAULT '';

ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS costing_sheet_id BIGINT DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS product_number INTEGER DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS equipment TEXT DEFAULT '';
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS model TEXT DEFAULT '';
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS specification TEXT DEFAULT '';
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS supplier TEXT DEFAULT '';
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS quantity DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS fob_eur DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS exchange_rate DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS total_cost_bhd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS markup_percent DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS selling_price_bhd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS total_suggested_bhd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE costing_line_items ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS costing_id TEXT DEFAULT '';
ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS customer_name TEXT DEFAULT '';
ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS project_name TEXT DEFAULT '';
ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS amount DOUBLE PRECISION DEFAULT 0;
ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'draft';
ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS pdf_path TEXT DEFAULT '';
ALTER TABLE offer_data ADD COLUMN IF NOT EXISTS sent_at TIMESTAMP WITH TIME ZONE;

-- Done
SELECT 'Migration complete. Tables verified.' AS status;
