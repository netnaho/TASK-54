package unit_tests

import (
	"strings"
	"testing"

	audithelper "careops/clinic/internal/shared/audit"
)

func TestAuditToJSON_Nil(t *testing.T) {
	result := audithelper.ToJSON(nil)
	if result != "{}" {
		t.Errorf("ToJSON(nil) = %q, want {}", result)
	}
}

func TestAuditToJSON_Struct(t *testing.T) {
	v := struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}{Name: "test", Count: 42}
	result := audithelper.ToJSON(v)
	if !strings.Contains(result, `"name":"test"`) {
		t.Errorf("ToJSON struct: want name field, got %s", result)
	}
	if !strings.Contains(result, `"count":42`) {
		t.Errorf("ToJSON struct: want count field, got %s", result)
	}
}

func TestAuditToJSON_Map(t *testing.T) {
	v := map[string]any{"action": "create", "entity": "users"}
	result := audithelper.ToJSON(v)
	if !strings.Contains(result, `"action"`) {
		t.Errorf("ToJSON map: want action key, got %s", result)
	}
}

func TestAuditToJSON_String(t *testing.T) {
	result := audithelper.ToJSON("plain string")
	if result != `"plain string"` {
		t.Errorf(`ToJSON("plain string") = %q, want "plain string"`, result)
	}
}
