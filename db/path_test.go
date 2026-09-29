package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agnosto/fansly-scraper/db/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dbDirMode); err != nil {
		t.Fatalf("failed to create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), dbFileMode); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func TestResolveDBPathPrefersExistingConfigDB(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	saveDir := t.TempDir()
	configDB := filepath.Join(configDir, dbFileName)
	saveDB := filepath.Join(saveDir, dbFileName)
	writeTestFile(t, configDB, "config")
	writeTestFile(t, saveDB, "save")

	got, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if got != configDB {
		t.Errorf("expected %s, got %s", configDB, got)
	}
	if got := readTestFile(t, saveDB); got != "save" {
		t.Errorf("save location database was modified: %q", got)
	}
	if _, err := os.Stat(saveDB + backupSuffix); !os.IsNotExist(err) {
		t.Errorf("unexpected backup created for a database that was not imported")
	}
}

func TestResolveDBPathCreatesInConfigDir(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "nested", "fansly-scraper")
	saveDir := t.TempDir()

	got, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if want := filepath.Join(configDir, dbFileName); got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
	if _, err := os.Stat(configDir); err != nil {
		t.Errorf("config directory was not created: %v", err)
	}
}

func TestResolveDBPathImportsSaveLocationDB(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	saveDB := filepath.Join(saveDir, dbFileName)
	writeTestFile(t, saveDB, "history")

	got, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if want := filepath.Join(configDir, dbFileName); got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
	if got := readTestFile(t, got); got != "history" {
		t.Errorf("imported database content = %q, want %q", got, "history")
	}
	if got := readTestFile(t, saveDB+backupSuffix); got != "history" {
		t.Errorf("backup content = %q, want %q", got, "history")
	}
	if _, err := os.Stat(saveDB); !os.IsNotExist(err) {
		t.Errorf("original database should have been moved aside, stat error = %v", err)
	}
}

func TestResolveDBPathImportRunsOnlyOnce(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	configDB := filepath.Join(configDir, dbFileName)
	writeTestFile(t, filepath.Join(saveDir, dbFileName), "old history")
	writeTestFile(t, configDB, "new history")

	got, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if got != configDB {
		t.Fatalf("expected %s, got %s", configDB, got)
	}
	if got := readTestFile(t, configDB); got != "new history" {
		t.Errorf("existing config database was overwritten: %q", got)
	}
}

func TestResolveDBPathImportsJournal(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	writeTestFile(t, filepath.Join(saveDir, dbFileName), "db")
	writeTestFile(t, filepath.Join(saveDir, dbFileName+journalSuffix), "journal")

	if _, err := resolveDBPath(configDir, saveDir); err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if got := readTestFile(t, filepath.Join(configDir, dbFileName+journalSuffix)); got != "journal" {
		t.Errorf("imported journal content = %q, want %q", got, "journal")
	}
	if got := readTestFile(t, filepath.Join(saveDir, dbFileName+backupSuffix+journalSuffix)); got != "journal" {
		t.Errorf("backup journal content = %q, want %q", got, "journal")
	}
}

func TestResolveDBPathReplacesStaleBackup(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	writeTestFile(t, filepath.Join(saveDir, dbFileName), "current")
	writeTestFile(t, filepath.Join(saveDir, dbFileName+backupSuffix), "stale")

	if _, err := resolveDBPath(configDir, saveDir); err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if got := readTestFile(t, filepath.Join(saveDir, dbFileName+backupSuffix)); got != "current" {
		t.Errorf("backup content = %q, want %q", got, "current")
	}
}

func TestResolveDBPathEmptySaveLocation(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")

	got, err := resolveDBPath(configDir, "")
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if want := filepath.Join(configDir, dbFileName); got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveDBPathEmptySaveLocationIgnoresWorkingDirectory(t *testing.T) {
	workDir := t.TempDir()
	writeTestFile(t, filepath.Join(workDir, dbFileName), "stray")
	t.Chdir(workDir)

	got, err := resolveDBPath(filepath.Join(t.TempDir(), "config"), "")
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected an absolute path, got %s", got)
	}
	if got := readTestFile(t, filepath.Join(workDir, dbFileName)); got != "stray" {
		t.Errorf("a database in the working directory was consumed: %q", got)
	}
}

func TestResolveDBPathFallsBackWhenConfigDirNotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	if err := os.MkdirAll(configDir, dbDirMode); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(saveDir, dbDirMode); err != nil {
		t.Fatalf("failed to create save dir: %v", err)
	}
	if err := os.Chmod(configDir, 0o500); err != nil {
		t.Fatalf("failed to make config dir read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(configDir, dbDirMode) })

	got, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if want := filepath.Join(saveDir, dbFileName); got != want {
		t.Errorf("expected fallback to %s, got %s", want, got)
	}
}

func TestResolveDBPathErrorsWhenNowhereWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	for _, dir := range []string{configDir, saveDir} {
		if err := os.MkdirAll(dir, dbDirMode); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("failed to make %s read-only: %v", dir, err)
		}
		t.Cleanup(func() { os.Chmod(dir, dbDirMode) })
	}

	if _, err := resolveDBPath(configDir, saveDir); err == nil {
		t.Fatal("expected an error when no location is writable, got nil")
	}
}

func TestResolveDBPathKeepsSaveDBWhenImportFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	saveDB := filepath.Join(saveDir, dbFileName)
	writeTestFile(t, saveDB, "history")
	if err := os.MkdirAll(configDir, dbDirMode); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.Chmod(configDir, 0o500); err != nil {
		t.Fatalf("failed to make config dir read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(configDir, dbDirMode) })

	got, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if got != saveDB {
		t.Errorf("expected fallback to %s, got %s", saveDB, got)
	}
	if got := readTestFile(t, saveDB); got != "history" {
		t.Errorf("save location database was damaged: %q", got)
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatalf("failed to read config dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), dbFileName+".import-") {
			t.Errorf("temporary import file was left behind: %s", entry.Name())
		}
	}
}

func TestDataSourceName(t *testing.T) {
	got := dataSourceName(filepath.Join("dir", dbFileName))
	want := filepath.Join("dir", dbFileName) + "?_pragma=busy_timeout(5000)"
	if got != want {
		t.Errorf("dataSourceName = %q, want %q", got, want)
	}

	withQuestionMark := filepath.Join("dir", "qu?estion", dbFileName)
	if got := dataSourceName(withQuestionMark); got != withQuestionMark {
		t.Errorf("dataSourceName = %q, want it left untouched as %q", got, withQuestionMark)
	}
}

// seedDatabase builds a real database at path with a file already recorded in
// it, the way an earlier version of the app would have left one behind in the
// download directory.
func seedDatabase(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), dbDirMode); err != nil {
		t.Fatalf("failed to create %s: %v", filepath.Dir(path), err)
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("failed to create seed database: %v", err)
	}
	if err := db.AutoMigrate(&models.File{}, &models.ProcessedPost{}); err != nil {
		t.Fatalf("failed to migrate seed database: %v", err)
	}
	if err := db.Create(&models.File{Model: "model", Hash: "hash", Path: "/tmp/a.mp4", FileType: "video"}).Error; err != nil {
		t.Fatalf("failed to seed database: %v", err)
	}
	if sqlDB, err := db.DB(); err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	} else if err := sqlDB.Close(); err != nil {
		t.Fatalf("failed to close seed database: %v", err)
	}
}

func TestImportedDatabaseIsUsable(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	saveDir := filepath.Join(root, "downloads")
	seedDatabase(t, filepath.Join(saveDir, dbFileName))

	dbPath, err := resolveDBPath(configDir, saveDir)
	if err != nil {
		t.Fatalf("resolveDBPath returned error: %v", err)
	}
	if want := filepath.Join(configDir, dbFileName); dbPath != want {
		t.Fatalf("resolveDBPath = %s, want %s", dbPath, want)
	}

	database, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("openDB returned error: %v", err)
	}
	defer database.Close()

	var count int64
	if err := database.DB.Model(&models.File{}).Where("hash = ?", "hash").Count(&count).Error; err != nil {
		t.Fatalf("failed to query the imported database: %v", err)
	}
	if count != 1 {
		t.Errorf("imported records = %d, want 1", count)
	}

	var timeout int
	if err := database.DB.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil {
		t.Fatalf("failed to read the busy timeout: %v", err)
	}
	if timeout != busyTimeoutMS {
		t.Errorf("busy_timeout = %d, want %d", timeout, busyTimeoutMS)
	}
}

// TestNewDatabaseImportsFromDownloadLocation exercises the whole path the app
// takes at startup, with the config directory redirected to a temporary one.
func TestNewDatabaseImportsFromDownloadLocation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the config directory is taken from %AppData% on Windows")
	}

	root := t.TempDir()
	configDir := filepath.Join(root, "xdg")
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("HOME", root)

	saveDir := filepath.Join(root, "downloads")
	saveDB := filepath.Join(saveDir, dbFileName)
	seedDatabase(t, saveDB)

	wantPath := filepath.Join(configDir, "fansly-scraper", dbFileName)
	for i := range 2 {
		database, err := NewDatabase(saveDir)
		if err != nil {
			t.Fatalf("NewDatabase returned error on run %d: %v", i+1, err)
		}
		if database.Path != wantPath {
			t.Errorf("run %d opened %s, want %s", i+1, database.Path, wantPath)
		}
		var count int64
		if err := database.DB.Model(&models.File{}).Count(&count).Error; err != nil {
			t.Fatalf("run %d failed to query the database: %v", i+1, err)
		}
		if count != 1 {
			t.Errorf("run %d found %d records, want 1", i+1, count)
		}
		if err := database.Close(); err != nil {
			t.Fatalf("run %d failed to close the database: %v", i+1, err)
		}
	}

	if _, err := os.Stat(saveDB); !os.IsNotExist(err) {
		t.Errorf("the download location database should have been moved aside, stat error = %v", err)
	}
	if _, err := os.Stat(saveDB + backupSuffix); err != nil {
		t.Errorf("expected a backup at %s: %v", saveDB+backupSuffix, err)
	}
}
