package hierarchy

import (
	"encoding/json"
	"testing"
)

func TestParsePermissions(t *testing.T) {
	raw := json.RawMessage(`{
		"allowedChannels": ["email", "whatsapp"],
		"allowedActions": ["read_email", "send_email"],
		"restrictedTopics": ["salary", "termination"],
		"maxSpendCents": 500
	}`)

	perms, err := ParsePermissions(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(perms.AllowedChannels) != 2 {
		t.Errorf("expected 2 allowed channels, got %d", len(perms.AllowedChannels))
	}
	if perms.MaxSpendCents != 500 {
		t.Errorf("expected max spend 500, got %d", perms.MaxSpendCents)
	}
}

func TestParsePermissionsEmpty(t *testing.T) {
	perms, err := ParsePermissions(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms.AllowedChannels) != 0 {
		t.Error("expected empty allowed channels")
	}
}

func TestCheckChannelAllowed(t *testing.T) {
	perms := &Permissions{AllowedChannels: []string{"email", "whatsapp"}}

	if check := CheckChannelAllowed(perms, "email"); !check.Allowed {
		t.Error("expected email to be allowed")
	}
	if check := CheckChannelAllowed(perms, "WhatsApp"); !check.Allowed {
		t.Error("expected WhatsApp to be allowed (case insensitive)")
	}
	if check := CheckChannelAllowed(perms, "phone"); check.Allowed {
		t.Error("expected phone to be blocked")
	}
}

func TestCheckChannelNoRestrictions(t *testing.T) {
	perms := &Permissions{}
	if check := CheckChannelAllowed(perms, "anything"); !check.Allowed {
		t.Error("expected all channels allowed when no restrictions set")
	}
}

func TestCheckActionAllowed(t *testing.T) {
	perms := &Permissions{AllowedActions: []string{"read_email", "send_email"}}

	if check := CheckActionAllowed(perms, "read_email"); !check.Allowed {
		t.Error("expected read_email allowed")
	}
	if check := CheckActionAllowed(perms, "delete_account"); check.Allowed {
		t.Error("expected delete_account blocked")
	}
}

func TestCheckTopicAllowed(t *testing.T) {
	perms := &Permissions{RestrictedTopics: []string{"salary", "termination"}}

	if check := CheckTopicAllowed(perms, "What is my salary?"); check.Allowed {
		t.Error("expected salary topic to be blocked")
	}
	if check := CheckTopicAllowed(perms, "I need help with my order"); !check.Allowed {
		t.Error("expected order topic to be allowed")
	}
	if check := CheckTopicAllowed(perms, "TERMINATION notice"); check.Allowed {
		t.Error("expected TERMINATION (case insensitive) to be blocked")
	}
}

func TestCheckSpendAllowed(t *testing.T) {
	perms := &Permissions{MaxSpendCents: 500}

	if check := CheckSpendAllowed(perms, 100); !check.Allowed {
		t.Error("expected 100 cents allowed under 500 limit")
	}
	if check := CheckSpendAllowed(perms, 600); check.Allowed {
		t.Error("expected 600 cents blocked over 500 limit")
	}
}

func TestCheckSpendNoLimit(t *testing.T) {
	perms := &Permissions{}
	if check := CheckSpendAllowed(perms, 99999); !check.Allowed {
		t.Error("expected no limit when MaxSpendCents is 0")
	}
}
