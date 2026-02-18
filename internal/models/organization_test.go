package models

import (
	"encoding/json"
	"testing"
)

func TestNullString(t *testing.T) {
	if nullString("") != nil {
		t.Error("expected nil for empty string")
	}
	if nullString("hello") != "hello" {
		t.Error("expected 'hello' for non-empty string")
	}
}

func TestDefaultJSON(t *testing.T) {
	result := defaultJSON(nil)
	if string(result) != "{}" {
		t.Errorf("expected '{}' for nil input, got '%s'", string(result))
	}

	result = defaultJSON(json.RawMessage{})
	if string(result) != "{}" {
		t.Errorf("expected '{}' for empty input, got '%s'", string(result))
	}

	input := json.RawMessage(`{"key":"value"}`)
	result = defaultJSON(input)
	if string(result) != `{"key":"value"}` {
		t.Errorf("expected preserved JSON, got '%s'", string(result))
	}
}
