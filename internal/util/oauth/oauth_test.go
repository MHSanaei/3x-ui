package oauth

import (
	"reflect"
	"testing"
)

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want []string
	}{
		{"array of strings", []string{"a", "b"}, []string{"a", "b"}},
		{"json array", []any{"a", "b"}, []string{"a", "b"}},
		{"json array with non-strings", []any{"a", 1, "", "b"}, []string{"a", "b"}},
		{"single string", "solo", []string{"solo"}},
		{"empty string", "", nil},
		{"nil", nil, nil},
		{"unexpected type", 42, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toStringSlice(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("toStringSlice(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractIdentity(t *testing.T) {
	tests := []struct {
		name          string
		subject       string
		claims        map[string]any
		usernameClaim string
		groupsClaim   string
		want          Identity
	}{
		{
			name:          "preferred_username claim",
			subject:       "sub-1",
			claims:        map[string]any{"preferred_username": "alice", "email": "alice@x.io", "groups": []any{"admins"}},
			usernameClaim: "preferred_username",
			groupsClaim:   "groups",
			want:          Identity{Subject: "sub-1", Username: "alice", Email: "alice@x.io", Groups: []string{"admins"}},
		},
		{
			name:          "username falls back to email",
			subject:       "sub-2",
			claims:        map[string]any{"email": "bob@x.io"},
			usernameClaim: "preferred_username",
			groupsClaim:   "groups",
			want:          Identity{Subject: "sub-2", Username: "bob@x.io", Email: "bob@x.io"},
		},
		{
			name:          "username falls back to subject",
			subject:       "sub-3",
			claims:        map[string]any{},
			usernameClaim: "email",
			groupsClaim:   "groups",
			want:          Identity{Subject: "sub-3", Username: "sub-3"},
		},
		{
			name:          "custom groups claim as roles",
			subject:       "sub-4",
			claims:        map[string]any{"email": "c@x.io", "roles": []any{"vpn", "staff"}},
			usernameClaim: "email",
			groupsClaim:   "roles",
			want:          Identity{Subject: "sub-4", Username: "c@x.io", Email: "c@x.io", Groups: []string{"vpn", "staff"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIdentity(tt.subject, tt.claims, tt.usernameClaim, tt.groupsClaim)
			if !reflect.DeepEqual(*got, tt.want) {
				t.Fatalf("extractIdentity = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestIdentityGroupMembership(t *testing.T) {
	id := &Identity{Groups: []string{"admins", "staff"}}
	if !id.InGroup("admins") {
		t.Error("InGroup(admins) = false, want true")
	}
	if id.InGroup("nope") {
		t.Error("InGroup(nope) = true, want false")
	}
	if id.InGroup("") {
		t.Error("InGroup(empty) = true, want false")
	}
	if !id.InAnyGroup([]string{"none", "staff"}) {
		t.Error("InAnyGroup([none staff]) = false, want true")
	}
	if id.InAnyGroup([]string{"none", "other"}) {
		t.Error("InAnyGroup([none other]) = true, want false")
	}
	if (&Identity{}).InAnyGroup(nil) {
		t.Error("empty identity InAnyGroup(nil) = true, want false")
	}
}

func TestNewFlowStateUnique(t *testing.T) {
	a, err := NewFlowState()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewFlowState()
	if err != nil {
		t.Fatal(err)
	}
	if a.State == "" || a.Nonce == "" || a.Verifier == "" {
		t.Fatalf("empty flow state field: %+v", a)
	}
	if a.State == b.State || a.Nonce == b.Nonce || a.Verifier == b.Verifier {
		t.Fatal("flow state fields are not unique across calls")
	}
}
