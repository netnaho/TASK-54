// Package audit provides helpers for constructing audit log entries.
package audit

import "encoding/json"

// ToJSON serialises v to a compact JSON string for use as a before/after
// snapshot in audit log entries. Returns "{}" on serialisation failure or
// when v is nil, so callers can always pass the result without a nil check.
func ToJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
