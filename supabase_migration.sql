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
ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS is_decision_maker BOOLEAN DEFAULT FALSE;
ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS decision_role TEXT DEFAULT '';
ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS reports_to_contact_id TEXT DEFAULT '';
ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS reports_to_name TEXT DEFAULT '';
ALTER TABLE supplier_contacts ADD COLUMN IF NOT EXISTS is_decision_maker BOOLEAN DEFAULT FALSE;
ALTER TABLE supplier_contacts ADD COLUMN IF NOT EXISTS decision_role TEXT DEFAULT '';
ALTER TABLE supplier_contacts ADD COLUMN IF NOT EXISTS reports_to_contact_id TEXT DEFAULT '';
ALTER TABLE supplier_contacts ADD COLUMN IF NOT EXISTS reports_to_name TEXT DEFAULT '';

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

-- Phase 39: User feedback hardening sync schema.
-- Expense efficiency fields used by the denser expense workspace.
ALTER TABLE expense_entries ADD COLUMN IF NOT EXISTS inventory_linked BOOLEAN DEFAULT FALSE;
ALTER TABLE expense_entries ADD COLUMN IF NOT EXISTS inventory_item_id TEXT;
ALTER TABLE expense_entries ADD COLUMN IF NOT EXISTS inventory_reference TEXT DEFAULT '';
ALTER TABLE expense_entries ADD COLUMN IF NOT EXISTS payment_method TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_expense_entries_inventory_item
    ON expense_entries(inventory_item_id);

-- Project commercial metadata used by the compact master-detail project editor.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS customer_name TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS end_user_name TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS opportunity_key TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS customer_poc_name TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS customer_poc_email TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS customer_poc_phone TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_projects_opportunity_key
    ON projects(opportunity_key);

-- Unified RFQ/opportunity/offer commercial thread tables.
CREATE TABLE IF NOT EXISTS rfq_comments (
    id BIGINT PRIMARY KEY,
    rfq_id BIGINT DEFAULT 0,
    comment TEXT DEFAULT '',
    created_by VARCHAR(100) DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS opportunity_comments (
    id BIGINT PRIMARY KEY,
    opportunity_id VARCHAR(36) DEFAULT '',
    comment TEXT DEFAULT '',
    created_by VARCHAR(100) DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS offer_notes (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    offer_id TEXT DEFAULT '',
    note_date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    content TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_rfq_comments_rfq_id
    ON rfq_comments(rfq_id);
CREATE INDEX IF NOT EXISTS idx_opportunity_comments_opportunity
    ON opportunity_comments(opportunity_id);
CREATE INDEX IF NOT EXISTS idx_offer_notes_offer_id
    ON offer_notes(offer_id);
CREATE INDEX IF NOT EXISTS idx_offer_notes_note_date
    ON offer_notes(note_date);

-- Admin-reviewed data quality queue. Review rows intentionally do not mutate
-- customer/opportunity/offer records; they record human decisions for sync/audit.
CREATE TABLE IF NOT EXISTS data_quality_reviews (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    issue_id TEXT UNIQUE NOT NULL,
    issue_type TEXT DEFAULT '',
    severity TEXT DEFAULT '',
    entity_type TEXT DEFAULT '',
    entity_id TEXT DEFAULT '',
    summary TEXT DEFAULT '',
    detail TEXT DEFAULT '',
    primary_action TEXT DEFAULT '',
    status TEXT DEFAULT '',
    review_note TEXT DEFAULT '',
    reviewed_by_id TEXT DEFAULT '',
    reviewed_by TEXT DEFAULT '',
    reviewed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_data_quality_reviews_status
    ON data_quality_reviews(status);
CREATE INDEX IF NOT EXISTS idx_data_quality_reviews_entity
    ON data_quality_reviews(entity_type, entity_id);

-- Admin-published data updates. These tables carry signed/hashed additive
-- update manifests from admin machines to deployed user PCs; normal sync only
-- transports the manifest/receipt records, while application code performs
-- backup + transactional allowlisted apply.
CREATE TABLE IF NOT EXISTS data_update_manifests (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    version_label TEXT,
    title TEXT DEFAULT '',
    description TEXT DEFAULT '',
    status TEXT DEFAULT 'draft',
    published_at TIMESTAMP WITH TIME ZONE,
    min_app_version TEXT DEFAULT '',
    manifest_hash TEXT DEFAULT '',
    apply_mode TEXT DEFAULT 'upsert_only',
    source TEXT DEFAULT ''
);

ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS version_label TEXT;
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS title TEXT DEFAULT '';
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'draft';
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS published_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS min_app_version TEXT DEFAULT '';
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS manifest_hash TEXT DEFAULT '';
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS apply_mode TEXT DEFAULT 'upsert_only';
ALTER TABLE data_update_manifests ADD COLUMN IF NOT EXISTS source TEXT DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_data_update_manifests_version_label
    ON data_update_manifests(version_label);
CREATE INDEX IF NOT EXISTS idx_data_update_manifests_status
    ON data_update_manifests(status);

CREATE TABLE IF NOT EXISTS data_update_operations (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    manifest_id TEXT DEFAULT '',
    sequence INTEGER DEFAULT 0,
    table_name TEXT DEFAULT '',
    operation TEXT DEFAULT 'upsert',
    primary_key TEXT DEFAULT '',
    row_json TEXT DEFAULT '',
    row_hash TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_data_update_operations_manifest
    ON data_update_operations(manifest_id, sequence);
CREATE INDEX IF NOT EXISTS idx_data_update_operations_table
    ON data_update_operations(table_name);

CREATE TABLE IF NOT EXISTS data_update_receipts (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    device_id TEXT DEFAULT '',
    manifest_id TEXT DEFAULT '',
    status TEXT DEFAULT '',
    backup_path TEXT DEFAULT '',
    applied_at TIMESTAMP WITH TIME ZONE,
    error TEXT DEFAULT '',
    operation_count INTEGER DEFAULT 0,
    manifest_hash TEXT DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_data_update_receipt_device_manifest
    ON data_update_receipts(device_id, manifest_id);
CREATE INDEX IF NOT EXISTS idx_data_update_receipts_status
    ON data_update_receipts(status);

-- Queryable intelligence sidecars generated by OneDrive/seed enrichment passes.
CREATE TABLE IF NOT EXISTS intelligence_opportunity_enrichment (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    opportunity_id TEXT,
    year INTEGER DEFAULT 0,
    source TEXT DEFAULT '',
    folder_number TEXT DEFAULT '',
    folder_name TEXT DEFAULT '',
    customer_id TEXT DEFAULT '',
    customer_name TEXT DEFAULT '',
    stage TEXT DEFAULT '',
    revenue_bhd DOUBLE PRECISION DEFAULT 0,
    offer_id TEXT DEFAULT '',
    offer_number TEXT DEFAULT '',
    quote_type TEXT DEFAULT '',
    revision_number INTEGER DEFAULT 0,
    prepared_by TEXT DEFAULT '',
    prepared_by_quality TEXT DEFAULT '',
    primary_instrument_class TEXT DEFAULT '',
    primary_instrument_signal TEXT DEFAULT '',
    canonical_product_type TEXT DEFAULT '',
    product_mix_json TEXT DEFAULT '',
    line_item_count INTEGER DEFAULT 0,
    line_total_bhd DOUBLE PRECISION DEFAULT 0,
    line_source TEXT DEFAULT '',
    has_linked_offer BOOLEAN DEFAULT FALSE,
    has_product_details BOOLEAN DEFAULT FALSE,
    enrichment_notes TEXT DEFAULT '',
    enriched_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS intelligence_line_item_enrichment (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    entity_kind TEXT DEFAULT '',
    entity_id TEXT DEFAULT '',
    source_line_kind TEXT DEFAULT '',
    source_line_id TEXT DEFAULT '',
    opportunity_id TEXT DEFAULT '',
    offer_id TEXT DEFAULT '',
    order_id TEXT DEFAULT '',
    line_number INTEGER DEFAULT 0,
    folder_number TEXT DEFAULT '',
    folder_name TEXT DEFAULT '',
    customer_name TEXT DEFAULT '',
    document_number TEXT DEFAULT '',
    description TEXT DEFAULT '',
    equipment TEXT DEFAULT '',
    model TEXT DEFAULT '',
    specification TEXT DEFAULT '',
    quantity DOUBLE PRECISION DEFAULT 0,
    unit_price_bhd DOUBLE PRECISION DEFAULT 0,
    total_price_bhd DOUBLE PRECISION DEFAULT 0,
    prepared_by TEXT DEFAULT '',
    instrument_class TEXT DEFAULT '',
    instrument_signal TEXT DEFAULT '',
    source_confidence TEXT DEFAULT '',
    enriched_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS intelligence_order_enrichment (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER DEFAULT 1,
    created_by TEXT DEFAULT '',
    order_id TEXT,
    year INTEGER DEFAULT 0,
    order_number TEXT DEFAULT '',
    customer_id TEXT DEFAULT '',
    customer_name TEXT DEFAULT '',
    order_date TEXT DEFAULT '',
    status TEXT DEFAULT '',
    grand_total_bhd DOUBLE PRECISION DEFAULT 0,
    primary_instrument_class TEXT DEFAULT '',
    primary_instrument_signal TEXT DEFAULT '',
    canonical_product_type TEXT DEFAULT '',
    product_mix_json TEXT DEFAULT '',
    line_item_count INTEGER DEFAULT 0,
    line_total_bhd DOUBLE PRECISION DEFAULT 0,
    enriched_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_intel_opp_enrich_year
    ON intelligence_opportunity_enrichment(year, source);
CREATE INDEX IF NOT EXISTS idx_intel_opp_enrich_customer
    ON intelligence_opportunity_enrichment(customer_name);
CREATE INDEX IF NOT EXISTS idx_intel_opp_enrich_instrument
    ON intelligence_opportunity_enrichment(primary_instrument_class, primary_instrument_signal);
CREATE INDEX IF NOT EXISTS idx_intel_line_enrich_entity
    ON intelligence_line_item_enrichment(entity_kind, entity_id);
CREATE INDEX IF NOT EXISTS idx_intel_line_enrich_instrument
    ON intelligence_line_item_enrichment(instrument_class, instrument_signal);
CREATE INDEX IF NOT EXISTS idx_intel_order_enrich_year
    ON intelligence_order_enrichment(year);
CREATE INDEX IF NOT EXISTS idx_intel_order_enrich_instrument
    ON intelligence_order_enrichment(primary_instrument_class, primary_instrument_signal);

-- Done
SELECT 'Migration complete. Tables verified.' AS status;
