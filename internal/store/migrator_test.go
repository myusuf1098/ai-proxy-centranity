package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/store"
)

func TestLoadMigrationFiles(t *testing.T) {
	tmpDir := t.TempDir()

	f1 := filepath.Join(tmpDir, "000001_init.up.sql")
	f2 := filepath.Join(tmpDir, "000002_add_cols.up.sql")
	f3 := filepath.Join(tmpDir, "000001_init.down.sql")

	_ = os.WriteFile(f1, []byte("CREATE TABLE t1 (id int);"), 0644)
	_ = os.WriteFile(f2, []byte("CREATE TABLE t2 (id int);"), 0644)
	_ = os.WriteFile(f3, []byte("DROP TABLE t1;"), 0644)

	files, err := store.FindUpMigrations(tmpDir)
	if err != nil {
		t.Fatalf("expected no error finding migrations, got: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 up migrations, got %d", len(files))
	}

	if filepath.Base(files[0]) != "000001_init.up.sql" {
		t.Errorf("expected first migration 000001_init.up.sql, got %s", files[0])
	}
}
