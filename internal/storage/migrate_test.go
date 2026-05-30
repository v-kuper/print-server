package storage

import (
	"strings"
	"testing"
)

func TestEmbeddedMigrationDefinesPersistenceTables(t *testing.T) {
	sql, err := MigrationSQLForTests()
	if err != nil {
		t.Fatalf("load migration SQL: %v", err)
	}

	for _, table := range []string{
		"workspaces",
		"users",
		"workspace_memberships",
		"printers",
		"workspace_settings",
		"scheduler_state",
		"image_editor_state",
		"google_tokens",
		"print_jobs",
		"receipt_snapshots",
		"receipt_snapshot_summaries",
		"audit_events",
		"legacy_imports",
	} {
		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("expected migration to create %s table", table)
		}
	}
	if !strings.Contains(sql, "value JSONB") || !strings.Contains(sql, "preview_png BYTEA") || !strings.Contains(sql, "receipt_lines JSONB") {
		t.Fatalf("expected migration to include JSONB settings and BYTEA image data:\n%s", sql)
	}
}
