package main

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

func TestManualMigrateSupabaseSchema(t *testing.T) {
	if os.Getenv("APPLY_SUPABASE_SCHEMA") != "1" {
		t.Skip("set APPLY_SUPABASE_SCHEMA=1 to migrate the configured Supabase schema")
	}

	require.NoError(t, godotenv.Overload("deploy_package/.env"))

	cfg := LoadSupabaseConfig()
	require.True(t, cfg.Enabled, "Supabase config must be enabled and complete")

	manager := NewDBManager(nil, cfg)
	require.NoError(t, manager.ConnectRemote())
	t.Cleanup(manager.Disconnect)

	require.NoError(t, manager.MigrateRemote())
}
