// Package xlsxutil writes minimal XLSX (Office Open XML) files without
// requiring an external dependency. An XLSX is a ZIP archive containing a
// fixed set of XML documents; this package produces a single-sheet workbook.
package xlsxutil

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile creates an XLSX file at path with the given sheet name, column
// headers, and data rows. Missing parent directories are created automatically.
func WriteFile(path, sheetName string, headers []string, rows [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xlsxutil: mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("xlsxutil: create: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Collect all strings for the shared string table.
	var allRows [][]string
	if len(headers) > 0 {
		allRows = append(allRows, headers)
	}
	allRows = append(allRows, rows...)

	strIndex := map[string]int{}
	var strs []string
	for _, row := range allRows {
		for _, cell := range row {
			if _, ok := strIndex[cell]; !ok {
				strIndex[cell] = len(strs)
				strs = append(strs, cell)
			}
		}
	}

	// [Content_Types].xml
	if err := writeZipEntry(zw, "[Content_Types].xml", contentTypesXML()); err != nil {
		return err
	}
	// _rels/.rels
	if err := writeZipEntry(zw, "_rels/.rels", relsXML()); err != nil {
		return err
	}
	// xl/workbook.xml
	if err := writeZipEntry(zw, "xl/workbook.xml", workbookXML(sheetName)); err != nil {
		return err
	}
	// xl/_rels/workbook.xml.rels
	if err := writeZipEntry(zw, "xl/_rels/workbook.xml.rels", workbookRelsXML()); err != nil {
		return err
	}
	// xl/styles.xml
	if err := writeZipEntry(zw, "xl/styles.xml", stylesXML()); err != nil {
		return err
	}
	// xl/sharedStrings.xml
	if err := writeZipEntry(zw, "xl/sharedStrings.xml", sharedStringsXML(strs)); err != nil {
		return err
	}
	// xl/worksheets/sheet1.xml
	sheetXML := worksheetXML(allRows, strIndex)
	return writeZipEntry(zw, "xl/worksheets/sheet1.xml", sheetXML)
}

func writeZipEntry(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("xlsxutil: zip entry %s: %w", name, err)
	}
	_, err = w.Write([]byte(content))
	return err
}

// — XML generators —

func contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml"
    ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml"
    ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/sharedStrings.xml"
    ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
  <Override PartName="/xl/styles.xml"
    ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`
}

func relsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
    Target="xl/workbook.xml"/>
</Relationships>`
}

func workbookXML(sheetName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
          xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="%s" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`, xmlEsc(sheetName))
}

func workbookRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
    Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"
    Target="sharedStrings.xml"/>
  <Relationship Id="rId3"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
    Target="styles.xml"/>
</Relationships>`
}

func stylesXML() string {
	// Minimal styles: one font, one fill, one border, one cell format.
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
  </cellXfs>
</styleSheet>`
}

func sharedStringsXML(strs []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
			`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`+
			` count="%d" uniqueCount="%d">`, len(strs), len(strs),
	))
	for _, s := range strs {
		sb.WriteString("<si><t>")
		sb.WriteString(xmlEsc(s))
		sb.WriteString("</t></si>")
	}
	sb.WriteString("</sst>")
	return sb.String()
}

func worksheetXML(rows [][]string, strIndex map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	sb.WriteString("<sheetData>")
	for ri, row := range rows {
		rowNum := ri + 1
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, rowNum))
		for ci, cell := range row {
			colRef := colName(ci)
			cellRef := fmt.Sprintf("%s%d", colRef, rowNum)
			idx := strIndex[cell]
			sb.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, cellRef, idx))
		}
		sb.WriteString("</row>")
	}
	sb.WriteString("</sheetData></worksheet>")
	return sb.String()
}

// colName converts a zero-based column index to a spreadsheet column label:
// 0→A, 1→B … 25→Z, 26→AA, 27→AB …
func colName(idx int) string {
	s := ""
	for idx >= 0 {
		s = string(rune('A'+idx%26)) + s
		idx = idx/26 - 1
	}
	return s
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
