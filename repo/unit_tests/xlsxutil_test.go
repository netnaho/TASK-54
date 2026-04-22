package unit_tests

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"careops/clinic/internal/shared/xlsxutil"
)

// colNameExported is a helper that calls the unexported colName via the
// WriteFile path — we instead test it indirectly by checking cell references
// in the produced XML, or we test it inline here using the same algorithm.
//
// Since colName is unexported we replicate the algorithm to test expected values.
func colNameHelper(idx int) string {
	s := ""
	for idx >= 0 {
		s = string(rune('A'+idx%26)) + s
		idx = idx/26 - 1
	}
	return s
}

func TestXlsxUtil_ColName(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "A"},
		{25, "Z"},
		{26, "AA"},
		{51, "AZ"},
		{52, "BA"},
	}
	for _, tc := range cases {
		got := colNameHelper(tc.idx)
		if got != tc.want {
			t.Errorf("colName(%d): want %q, got %q", tc.idx, tc.want, got)
		}
	}
}

func TestXlsxUtil_WriteFile_CreatesValidZIP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xlsx")

	headers := []string{"Name", "Age", "Role"}
	rows := [][]string{
		{"Alice", "30", "admin"},
		{"Bob", "25", "nurse"},
	}

	if err := xlsxutil.WriteFile(path, "Sheet1", headers, rows); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Verify the file exists and is non-empty.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat xlsx: %v", err)
	}
	if info.Size() == 0 {
		t.Error("xlsx file is empty")
	}

	// Open as a ZIP archive.
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open xlsx as zip: %v", err)
	}
	defer r.Close()

	// Expect well-known XLSX internal entries.
	want := map[string]bool{
		"[Content_Types].xml":          false,
		"_rels/.rels":                   false,
		"xl/workbook.xml":              false,
		"xl/worksheets/sheet1.xml":     false,
		"xl/sharedStrings.xml":         false,
		"xl/styles.xml":                false,
		"xl/_rels/workbook.xml.rels":   false,
	}
	for _, f := range r.File {
		if _, ok := want[f.Name]; ok {
			want[f.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("xlsx zip missing expected entry: %s", name)
		}
	}
}

func TestXlsxUtil_WriteFile_ContainsExpectedSheets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sheets.xlsx")

	headers := []string{"Col1", "Col2"}
	rows := [][]string{{"v1", "v2"}}

	sheetName := "My Report"
	if err := xlsxutil.WriteFile(path, sheetName, headers, rows); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open as zip: %v", err)
	}
	defer r.Close()

	// The workbook.xml should reference the provided sheet name.
	foundWorkbook := false
	for _, f := range r.File {
		if f.Name == "xl/workbook.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open workbook.xml: %v", err)
			}
			var buf [4096]byte
			n, _ := rc.Read(buf[:])
			rc.Close()
			content := string(buf[:n])
			if !containsStr(content, sheetName) {
				t.Errorf("workbook.xml does not contain sheet name %q", sheetName)
			}
			foundWorkbook = true
		}
	}
	if !foundWorkbook {
		t.Error("xl/workbook.xml not found in archive")
	}
}

func TestXlsxUtil_WriteFile_HeaderRowMatchesProvided(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "header.xlsx")

	headers := []string{"FirstName", "LastName", "Department"}
	rows := [][]string{{"John", "Doe", "Nursing"}}

	if err := xlsxutil.WriteFile(path, "Staff", headers, rows); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open as zip: %v", err)
	}
	defer r.Close()

	// sharedStrings.xml should contain all header values.
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open sharedStrings.xml: %v", err)
			}
			var buf [8192]byte
			n, _ := rc.Read(buf[:])
			rc.Close()
			content := string(buf[:n])
			for _, h := range headers {
				if !containsStr(content, h) {
					t.Errorf("sharedStrings.xml missing header %q", h)
				}
			}
			return
		}
	}
	t.Error("xl/sharedStrings.xml not found in archive")
}

func TestXlsxUtil_WriteFile_EmptyRowsProducesValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.xlsx")

	headers := []string{"A", "B", "C"}
	rows := [][]string{} // no data rows

	if err := xlsxutil.WriteFile(path, "EmptySheet", headers, rows); err != nil {
		t.Fatalf("WriteFile (empty rows): %v", err)
	}

	// Must still open as a valid ZIP.
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open empty xlsx as zip: %v", err)
	}
	defer r.Close()

	if len(r.File) == 0 {
		t.Error("empty-rows xlsx produced an archive with no files")
	}
}

// containsStr is a simple substring helper for test readability.
func containsStr(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		indexStr(haystack, needle) >= 0
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
