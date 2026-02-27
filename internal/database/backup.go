package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backup creates a standalone SQLite backup using VACUUM INTO.
// The output file is named broadwave-YYYYMMDD-HHMMSS.db in dir.
func Backup(db *sql.DB, dir string) error {
	filename := fmt.Sprintf("broadwave-%s.db", time.Now().Format("20060102-150405"))
	dest := filepath.Join(dir, filename)

	_, err := db.Exec(`VACUUM INTO ?`, dest)
	if err != nil {
		return fmt.Errorf("vacuum into %s: %w", dest, err)
	}

	log.Printf("Backup created: %s", dest)
	return nil
}

// PruneBackups removes broadwave-*.db files in dir older than keepDays days.
func PruneBackups(dir string, keepDays int) error {
	cutoff := time.Now().AddDate(0, 0, -keepDays)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading backup dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "broadwave-") || !strings.HasSuffix(name, ".db") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil {
				log.Printf("Error removing old backup %s: %v", path, err)
			} else {
				log.Printf("Pruned old backup: %s", path)
			}
		}
	}

	return nil
}

// RunBackup performs a backup and prunes files older than 7 days.
func RunBackup(db *sql.DB, dir string) {
	if err := Backup(db, dir); err != nil {
		log.Printf("Backup failed: %v", err)
		return
	}
	if err := PruneBackups(dir, 7); err != nil {
		log.Printf("Backup pruning failed: %v", err)
	}
}
