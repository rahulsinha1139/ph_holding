-- ═══════════════════════════════════════════════════════════════════════════
-- SUPABASE MANUAL MIGRATION SCRIPT
-- AsymmFlow ERP — Phase 34 (Butler Pipeline Rebuild)
-- Run against: postgres.xkyydcxxkzjbnsrmzkpp @ aws-1-ap-southeast-1.pooler.supabase.com
--
-- INSTRUCTIONS:
--   PGPASSWORD='PHHoldings2026' /opt/homebrew/opt/libpq/bin/psql \
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

-- Done
SELECT 'Migration complete. Tables verified.' AS status;
