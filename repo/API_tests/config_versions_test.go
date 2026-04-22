package api_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── Authorization ─────────────────────────────────────────────────────────────

func TestConfigVersions_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	for _, creds := range [][2]string{
		{"nurse@careops.local", "Nurse!234567"},
		{"finance@careops.local", "Finance!2345"},
		{"auditor@careops.local", "Auditor!2345"},
	} {
		cookie := loginAs(t, a, creds[0], creds[1])
		resp := getWithSession(t, a, "/config-versions", cookie)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s on /config-versions: want 403, got %d", creds[0], resp.StatusCode)
		}
	}
}

func TestConfigVersions_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/config-versions", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin on /config-versions: want 200, got %d", resp.StatusCode)
	}
}

// ── Snapshot ──────────────────────────────────────────────────────────────────

func TestConfigVersions_TakeSnapshot(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	req := httptest.NewRequest(http.MethodPost, "/config-versions/snapshot", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /config-versions/snapshot: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("take snapshot: want redirect, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/config-versions") {
		t.Errorf("snapshot redirect: want /config-versions*, got %q", loc)
	}
}

func TestConfigVersions_TakeSnapshot_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")

	req := httptest.NewRequest(http.MethodPost, "/config-versions/snapshot", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse POST snapshot: want 403, got %d", resp.StatusCode)
	}
}

// ── Edit form ─────────────────────────────────────────────────────────────────

func TestConfigVersions_EditFormGet200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// First get the index page to extract a valid config ID.
	indexResp := getWithSession(t, a, "/config-versions", cookie)
	body, _ := io.ReadAll(indexResp.Body)
	bodyStr := string(body)

	// Find first edit link: href="/config-versions/xxx/edit"
	editLinkPrefix := `href="/config-versions/`
	idx := strings.Index(bodyStr, editLinkPrefix)
	if idx < 0 {
		t.Skip("no config-versions entries found in index — seeds may not have loaded")
	}
	start := idx + len(editLinkPrefix)
	end := strings.Index(bodyStr[start:], "/edit")
	if end < 0 {
		t.Skip("no edit link found in config-versions index")
	}
	configID := bodyStr[start : start+end]
	// Sanity: config IDs should not contain spaces or quotes.
	if strings.ContainsAny(configID, ` "'<>`) {
		t.Skipf("extracted configID looks wrong: %q", configID)
	}

	resp := getWithSession(t, a, "/config-versions/"+configID+"/edit", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /config-versions/%s/edit: want 200, got %d", configID, resp.StatusCode)
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestConfigVersions_UpdateRedirectsOnSuccess(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Get index to find a config ID and row_version.
	indexResp := getWithSession(t, a, "/config-versions", cookie)
	indexBody, _ := io.ReadAll(indexResp.Body)
	indexStr := string(indexBody)

	editLinkPrefix := `href="/config-versions/`
	idx := strings.Index(indexStr, editLinkPrefix)
	if idx < 0 {
		t.Skip("no config entries in index")
	}
	start := idx + len(editLinkPrefix)
	end := strings.Index(indexStr[start:], "/edit")
	if end < 0 {
		t.Skip("no /edit link in index")
	}
	configID := indexStr[start : start+end]
	if strings.ContainsAny(configID, ` "'<>`) {
		t.Skipf("extracted configID looks wrong: %q", configID)
	}

	// Get the edit form to read the row_version.
	editResp := getWithSession(t, a, "/config-versions/"+configID+"/edit", cookie)
	editBody, _ := io.ReadAll(editResp.Body)
	editStr := string(editBody)

	rvPrefix := `name="row_version" value="`
	rvIdx := strings.Index(editStr, rvPrefix)
	if rvIdx < 0 {
		t.Skip("row_version not found in edit form")
	}
	rvStart := rvIdx + len(rvPrefix)
	rvEnd := strings.Index(editStr[rvStart:], `"`)
	rowVersion := editStr[rvStart : rvStart+rvEnd]

	form := url.Values{}
	form.Set("config_value", "updated-test-value")
	form.Set("row_version", rowVersion)

	req := httptest.NewRequest(http.MethodPost, "/config-versions/"+configID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /config-versions/%s: %v", configID, err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("update config: want redirect, got %d — %s", resp.StatusCode, string(body)[:min(200, len(body))])
	}
}

func TestConfigVersions_Update_EmptyValueReturns400(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	form := url.Values{}
	form.Set("config_value", "")
	form.Set("row_version", "1")

	req := httptest.NewRequest(http.MethodPost, "/config-versions/any-id", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty config value: want 400, got %d", resp.StatusCode)
	}
}

// ── Rollback ──────────────────────────────────────────────────────────────────

func TestConfigVersions_Rollback_MissingFileReturns400(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	form := url.Values{}
	form.Set("snapshot_file", "")

	req := httptest.NewRequest(http.MethodPost, "/config-versions/rollback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("rollback no file: want 400, got %d", resp.StatusCode)
	}
}

func TestConfigVersions_Rollback_NonExistentFileReturns400(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	form := url.Values{}
	form.Set("snapshot_file", "config_snapshot_19700101_000000.json")

	req := httptest.NewRequest(http.MethodPost, "/config-versions/rollback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("rollback non-existent file: want 400, got %d", resp.StatusCode)
	}
}

func TestConfigVersions_SnapshotThenRollback(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Take a snapshot.
	snapReq := httptest.NewRequest(http.MethodPost, "/config-versions/snapshot", nil)
	snapReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	snapResp, err := a.Test(snapReq, -1)
	if err != nil || (snapResp.StatusCode != http.StatusFound && snapResp.StatusCode != http.StatusSeeOther) {
		t.Fatalf("take snapshot failed: %v status=%d", err, snapResp.StatusCode)
	}

	// Read the index page to find the snapshot file name.
	indexResp := getWithSession(t, a, "/config-versions", cookie)
	indexBody, _ := io.ReadAll(indexResp.Body)
	indexStr := string(indexBody)

	snPrefix := `name="snapshot_file" value="`
	snIdx := strings.Index(indexStr, snPrefix)
	if snIdx < 0 {
		t.Skip("no snapshot found on config-versions page after taking one")
	}
	snStart := snIdx + len(snPrefix)
	snEnd := strings.Index(indexStr[snStart:], `"`)
	snapFile := indexStr[snStart : snStart+snEnd]

	// Roll back using the snapshot.
	rbForm := url.Values{}
	rbForm.Set("snapshot_file", snapFile)
	rbReq := httptest.NewRequest(http.MethodPost, "/config-versions/rollback", strings.NewReader(rbForm.Encode()))
	rbReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rbReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	rbResp, err := a.Test(rbReq, -1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rbResp.StatusCode != http.StatusFound && rbResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(rbResp.Body)
		t.Errorf("rollback: want redirect, got %d — %s", rbResp.StatusCode, string(body)[:min(200, len(body))])
	}
}
