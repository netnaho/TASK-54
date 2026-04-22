package unit_tests

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"careops/clinic/internal/shared/ziputil"
)

func TestZipUtil_Create_ProducesValidZIP(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "archive.zip")

	entries := []ziputil.Entry{
		{ArchiveName: "hello.txt", Content: []byte("hello world")},
	}

	if err := ziputil.Create(dest, entries); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify the file exists and can be opened as a ZIP.
	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Errorf("expected 1 entry in zip, got %d", len(r.File))
	}
}

func TestZipUtil_Create_ContainsAllEntries(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "multi.zip")

	entries := []ziputil.Entry{
		{ArchiveName: "a.txt", Content: []byte("file a")},
		{ArchiveName: "b.txt", Content: []byte("file b")},
		{ArchiveName: "subdir/c.txt", Content: []byte("file c in subdir")},
	}

	if err := ziputil.Create(dest, entries); err != nil {
		t.Fatalf("Create: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != len(entries) {
		t.Errorf("zip entry count: want %d, got %d", len(entries), len(r.File))
	}

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}
	for _, e := range entries {
		if !names[e.ArchiveName] {
			t.Errorf("entry %q not found in zip", e.ArchiveName)
		}
	}
}

func TestZipUtil_Create_EntryContentMatches(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "content.zip")

	payload := []byte("the quick brown fox jumps over the lazy dog")
	entries := []ziputil.Entry{
		{ArchiveName: "fox.txt", Content: payload},
	}

	if err := ziputil.Create(dest, entries); err != nil {
		t.Fatalf("Create: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "fox.txt" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open entry: %v", err)
			}
			got, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("read entry: %v", err)
			}
			if !bytes.Equal(got, payload) {
				t.Errorf("content mismatch: want %q, got %q", payload, got)
			}
			return
		}
	}
	t.Error("fox.txt not found in archive")
}

func TestZipUtil_Create_SourcePathEntry(t *testing.T) {
	dir := t.TempDir()

	// Create a temporary source file.
	srcPath := filepath.Join(dir, "source.txt")
	srcContent := []byte("content from source file")
	if err := os.WriteFile(srcPath, srcContent, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	dest := filepath.Join(dir, "from_source.zip")
	entries := []ziputil.Entry{
		{ArchiveName: "included.txt", SourcePath: srcPath},
	}

	if err := ziputil.Create(dest, entries); err != nil {
		t.Fatalf("Create with SourcePath: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "included.txt" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open entry: %v", err)
			}
			got, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("read entry: %v", err)
			}
			if !bytes.Equal(got, srcContent) {
				t.Errorf("source content mismatch: want %q, got %q", srcContent, got)
			}
			return
		}
	}
	t.Error("included.txt not found in archive")
}

func TestZipUtil_Create_EmptyEntriesProducesValidZIP(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "empty.zip")

	if err := ziputil.Create(dest, []ziputil.Entry{}); err != nil {
		t.Fatalf("Create (empty): %v", err)
	}

	// Must still be a valid (empty) ZIP.
	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open empty zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 0 {
		t.Errorf("empty zip: expected 0 files, got %d", len(r.File))
	}
}

func TestZipUtil_Create_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "nested", "deep", "archive.zip")

	entries := []ziputil.Entry{
		{ArchiveName: "note.txt", Content: []byte("nested")},
	}

	if err := ziputil.Create(dest, entries); err != nil {
		t.Fatalf("Create (nested path): %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("archive not created at nested path: %v", err)
	}
}
