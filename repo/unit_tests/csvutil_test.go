package unit_tests

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"careops/clinic/internal/shared/csvutil"
)

func TestCSVUtil_WriteFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.csv")

	headers := []string{"ID", "Name", "Amount"}
	rows := [][]string{
		{"1", "Alice", "100.00"},
		{"2", "Bob", "200.00"},
	}

	if err := csvutil.WriteFile(path, headers, rows); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("csv file is empty")
	}
}

func TestCSVUtil_WriteFile_CorrectRowCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rows.csv")

	headers := []string{"Col1", "Col2"}
	rows := [][]string{
		{"a", "b"},
		{"c", "d"},
		{"e", "f"},
	}

	if err := csvutil.WriteFile(path, headers, rows); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	// Expect 1 header row + 3 data rows = 4 total.
	wantTotal := 1 + len(rows)
	if len(records) != wantTotal {
		t.Errorf("row count: want %d, got %d", wantTotal, len(records))
	}

	// Verify header row.
	if len(records) > 0 {
		for i, h := range headers {
			if i >= len(records[0]) || records[0][i] != h {
				t.Errorf("header[%d]: want %q, got %q", i, h, records[0][i])
			}
		}
	}
}

func TestCSVUtil_WriteFile_CommaEscaping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "escaped.csv")

	headers := []string{"Name", "Description"}
	rows := [][]string{
		{"Test Product", "Price: 10,000 dollars"},
		{"Another", "No special chars"},
	}

	if err := csvutil.WriteFile(path, headers, rows); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll (comma escaping): %v — csv library should handle quoting", err)
	}

	// Row 1 (index 1, since index 0 is the header) should have the comma-containing value.
	if len(records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(records))
	}
	if records[1][1] != "Price: 10,000 dollars" {
		t.Errorf("comma value: want %q, got %q", "Price: 10,000 dollars", records[1][1])
	}
}

func TestCSVUtil_WriteFile_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	// Use a deeply nested path that does not exist yet.
	path := filepath.Join(dir, "deep", "nested", "report.csv")

	headers := []string{"X"}
	rows := [][]string{{"1"}}

	if err := csvutil.WriteFile(path, headers, rows); err != nil {
		t.Fatalf("WriteFile (nested dir): %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created at nested path: %v", err)
	}
}

func TestCSVUtil_WriteFile_NoDataRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "header_only.csv")

	headers := []string{"A", "B", "C"}
	rows := [][]string{} // empty

	if err := csvutil.WriteFile(path, headers, rows); err != nil {
		t.Fatalf("WriteFile (no rows): %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	// Should have exactly 1 row: the header.
	if len(records) != 1 {
		t.Errorf("header-only csv: want 1 record, got %d", len(records))
	}
}
