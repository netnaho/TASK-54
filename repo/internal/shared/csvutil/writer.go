// Package csvutil provides simple CSV writing helpers for report and export flows.
package csvutil

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile writes headers and rows to path, creating any missing parent
// directories. The file is truncated if it already exists.
func WriteFile(path string, headers []string, rows [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("csvutil: mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("csvutil: create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if len(headers) > 0 {
		if err := w.Write(headers); err != nil {
			return fmt.Errorf("csvutil: write headers: %w", err)
		}
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("csvutil: write row: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}
