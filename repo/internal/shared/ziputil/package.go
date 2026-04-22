// Package ziputil builds ZIP archives for the diagnostics export flow.
package ziputil

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Entry describes one file to include in the archive.
type Entry struct {
	// ArchiveName is the name the file appears under inside the ZIP.
	ArchiveName string
	// Content is the verbatim bytes to write. If Content is non-empty,
	// SourcePath is ignored.
	Content []byte
	// SourcePath is the on-disk file to copy; used when Content is empty.
	SourcePath string
}

// Create writes a ZIP archive to destPath containing the given entries.
// Missing parent directories are created automatically.
func Create(destPath string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("ziputil: mkdir: %w", err)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("ziputil: create %s: %w", destPath, err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, e := range entries {
		if err := addEntry(zw, e); err != nil {
			return err
		}
	}
	return nil
}

func addEntry(zw *zip.Writer, e Entry) error {
	w, err := zw.Create(e.ArchiveName)
	if err != nil {
		return fmt.Errorf("ziputil: entry %s: %w", e.ArchiveName, err)
	}
	if len(e.Content) > 0 {
		_, err = w.Write(e.Content)
		return err
	}
	if e.SourcePath == "" {
		return nil
	}
	src, err := os.Open(e.SourcePath)
	if err != nil {
		return fmt.Errorf("ziputil: open %s: %w", e.SourcePath, err)
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}
