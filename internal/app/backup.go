package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var backupMu sync.Mutex

func (s *Server) StartAutoBackup(defaultDir string, fallbackInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(fallbackInterval)
		defer ticker.Stop()
		_, _ = s.BackupNow(defaultDir, 20)
		for range ticker.C {
			settings, err := s.readSettings()
			if err != nil {
				continue
			}
			dir := settings.BackupDir
			if dir == "" {
				dir = defaultDir
			}
			_, _ = s.BackupNow(dir, settings.BackupKeepCount)
		}
	}()
}

func (s *Server) afterMutation() {
	settings, err := s.readSettings()
	if err != nil {
		return
	}
	go func() {
		_, _ = s.BackupNow(settings.BackupDir, settings.BackupKeepCount)
	}()
}

func (s *Server) BackupNow(dir string, keepCount int) (string, error) {
	backupMu.Lock()
	defer backupMu.Unlock()

	if dir == "" {
		dir = filepath.Join(filepath.Dir(s.dbPath), "backups")
	}
	if keepCount <= 0 {
		keepCount = 20
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	name := "classpoints-" + time.Now().Format("20060102-150405") + ".db"
	target := filepath.Join(dir, name)
	if err := copyFile(s.dbPath, target); err != nil {
		return "", err
	}
	_ = pruneBackups(dir, keepCount)
	return target, nil
}

func copyFile(source string, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func pruneBackups(dir string, keepCount int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "classpoints-") && strings.HasSuffix(entry.Name(), ".db") {
			files = append(files, entry)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
	if len(files) <= keepCount {
		return nil
	}
	for _, entry := range files[keepCount:] {
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return fmt.Errorf("删除旧备份失败: %w", err)
		}
	}
	return nil
}
