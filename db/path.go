package db

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/logger"
)

const (
	// dbFileName is the name of the SQLite database file.
	dbFileName = "downloads.db"
	// backupSuffix is appended to the original database when it is imported
	// from a download location into the config directory.
	backupSuffix = ".bak"
	// journalSuffix is the rollback journal SQLite keeps next to the database.
	journalSuffix = "-journal"
	// dbFileMode is deliberately owner-only: the database sits next to the
	// auth_token in config.toml.
	dbFileMode = 0o600
	// dbDirMode matches the permissions used for the rest of the config dir.
	dbDirMode = 0o755
)

// dbLog writes to the application logger when one has been initialized.
// logger.Logger is a nil *log.Logger until logger.InitLogger runs, which
// happens after the config is loaded, so it can legitimately be nil here.
func dbLog(format string, args ...any) {
	if logger.Logger != nil {
		logger.Logger.Printf(format, args...)
	}
}

// ResolveDBPath returns the path the database should be opened from.
//
// The config directory is the canonical location: SQLite relies on POSIX
// advisory locks that network shares (SMB in particular) do not relay
// reliably, which makes a database in the download directory fail with
// "database is locked (5) (SQLITE_BUSY)".
//
// A database left behind in the download location by an earlier version is
// imported into the config directory once, and the original is kept as
// downloads.db.bak. From then on the config directory copy is the only one
// that is ever used, so the downloader and the monitoring service always
// agree on the path.
func ResolveDBPath(saveLocation string) (string, error) {
	return resolveDBPath(config.GetConfigDir(), saveLocation)
}

func resolveDBPath(configDir, saveLocation string) (string, error) {
	configPath := filepath.Join(configDir, dbFileName)
	if fileExists(configPath) {
		return configPath, nil
	}

	// An empty save location would join to a path relative to the working
	// directory, so only look there when there is somewhere to look.
	if saveLocation != "" {
		savePath := filepath.Join(saveLocation, dbFileName)
		if fileExists(savePath) {
			if err := importDB(savePath, configPath); err != nil {
				// Fall back to the database already in the download location so
				// a broken import never leaves us without a database at all.
				dbLog("[WARN] Failed to import database from %s to %s: %v", savePath, configPath, err)
				dbLog("[WARN] Continuing with the existing database at %s", savePath)
				return savePath, nil
			}
			dbLog("[INFO] Imported existing database from %s to %s (original kept as %s)",
				savePath, configPath, savePath+backupSuffix)
			return configPath, nil
		}
	}

	if dirWritable(configDir) {
		return configPath, nil
	}

	if saveLocation == "" {
		return "", fmt.Errorf("cannot create database: config directory %s is not writable", configDir)
	}

	if !dirWritable(saveLocation) {
		return "", fmt.Errorf("cannot create database: neither %s nor %s is writable", configDir, saveLocation)
	}

	savePath := filepath.Join(saveLocation, dbFileName)
	dbLog("[WARN] Config directory %s is not writable, storing database in the download directory %s", configDir, savePath)
	return savePath, nil
}

// importDB copies the database at src to dst and moves the original to a
// .bak file next to it. The copy lands on a temporary file first and is
// published with a single rename so a crash mid-copy can never leave a
// truncated database at dst.
func importDB(src, dst string) error {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, dbDirMode); err != nil {
		return fmt.Errorf("failed to create %s: %w", dstDir, err)
	}

	tmp, err := os.CreateTemp(dstDir, dbFileName+".import-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}

	if err := copyDBFile(src, tmpPath, tmp); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to flush copied database: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close copied database: %w", err)
	}
	if err := os.Chmod(tmpPath, dbFileMode); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions on copied database: %w", err)
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move database into place: %w", err)
	}

	// A rollback journal is part of the database state: a database copied
	// without it can be left mid-transaction. The source is being moved aside
	// rather than deleted, so an unclean shutdown still has a way back.
	if fileExists(src + journalSuffix) {
		if err := copyDBFile(src+journalSuffix, dst+journalSuffix, nil); err != nil {
			dbLog("[WARN] Could not copy %s to %s: %v", src+journalSuffix, dst+journalSuffix, err)
		}
	}

	backup := src + backupSuffix
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		dbLog("[WARN] Could not replace old backup %s: %v", backup, err)
	}
	if err := os.Rename(src, backup); err != nil {
		dbLog("[WARN] Could not move the original database to %s: %v (it is now unused)", backup, err)
	}
	if fileExists(src + journalSuffix) {
		os.Remove(backup + journalSuffix)
		if err := os.Rename(src+journalSuffix, backup+journalSuffix); err != nil {
			dbLog("[WARN] Could not move %s to %s: %v", src+journalSuffix, backup+journalSuffix, err)
		}
	}

	return nil
}

// copyDBFile copies src over dst. When dst is non-nil it is written through
// that already-open file, otherwise dst is created and closed.
func copyDBFile(src, dst string, dstFile *os.File) (err error) {
	if !fileExists(src) {
		return fmt.Errorf("%s does not exist", src)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer in.Close()

	if dstFile == nil {
		dstFile, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, dbFileMode)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", dst, err)
		}
		defer func() {
			if cerr := dstFile.Close(); err == nil && cerr != nil {
				err = fmt.Errorf("failed to close %s: %w", dst, cerr)
			}
		}()
	}

	if _, err := io.Copy(dstFile, in); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
	}
	return nil
}

// fileExists reports whether path is an existing regular file. A directory
// named like the database is not one.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// dirWritable reports whether dir exists (or can be created) and accepts new
// files.
func dirWritable(dir string) bool {
	if err := os.MkdirAll(dir, dbDirMode); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".fansly-scraper-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
