package geodata

import (
	"errors"
	"testing"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		kind    GeoKind
		want    Reference
		wantErr error
	}{
		{
			name:  "geosite shorthand",
			token: "geosite:google",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: "google"},
		},
		{
			name:  "geosite code is case insensitive",
			token: "geosite:GOOGLE",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: "google"},
		},
		{
			name:  "geosite with attribute",
			token: "geosite:google@ads",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: "google", Attributes: []string{"ads"}},
		},
		{
			name:  "geosite with several attributes",
			token: "geosite:google@ads@cn",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: "google", Attributes: []string{"ads", "cn"}},
		},
		{
			name:  "ext form",
			token: "ext:my_rules.dat:corp",
			kind:  KindSite,
			want:  Reference{File: "my_rules.dat", Code: "corp"},
		},
		{
			name:  "ext-site form",
			token: "ext-site:my_rules.dat:corp",
			kind:  KindSite,
			want:  Reference{File: "my_rules.dat", Code: "corp"},
		},
		{
			name:  "ext-domain form",
			token: "ext-domain:my_rules.dat:corp",
			kind:  KindSite,
			want:  Reference{File: "my_rules.dat", Code: "corp"},
		},
		{
			name:  "surrounding spaces",
			token: "  geosite:google  ",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: "google"},
		},
		{
			name:  "plain domain needs no database",
			token: "google.com",
			kind:  KindSite,
			want:  Reference{},
		},
		{
			name:  "domain keyword rule needs no database",
			token: "keyword:google",
			kind:  KindSite,
			want:  Reference{},
		},
		{
			name:  "geoip shorthand",
			token: "geoip:private",
			kind:  KindIP,
			want:  Reference{File: "geoip.dat", Code: "private"},
		},
		{
			name:  "geoip reverse before the prefix",
			token: "!geoip:cn",
			kind:  KindIP,
			want:  Reference{File: "geoip.dat", Code: "cn", Reverse: true},
		},
		{
			name:  "geoip reverse before the code",
			token: "geoip:!cn",
			kind:  KindIP,
			want:  Reference{File: "geoip.dat", Code: "cn", Reverse: true},
		},
		{
			name:  "ext-ip form",
			token: "ext-ip:my_ips.dat:office",
			kind:  KindIP,
			want:  Reference{File: "my_ips.dat", Code: "office"},
		},
		{
			name:  "plain cidr needs no database",
			token: "10.0.0.0/8",
			kind:  KindIP,
			want:  Reference{},
		},
		{name: "empty token", token: "   ", kind: KindSite, wantErr: ErrInvalidToken},
		{name: "ext without code", token: "ext:geosite.dat", kind: KindSite, wantErr: ErrInvalidToken},
		{name: "ext with empty code", token: "ext:geosite.dat:", kind: KindSite, wantErr: ErrInvalidToken},
		{name: "ext with empty file", token: "ext::google", kind: KindSite, wantErr: ErrInvalidToken},
		{name: "geosite without code", token: "geosite:", kind: KindSite, wantErr: ErrInvalidToken},
		{name: "bare ext prefix", token: "ext:", kind: KindSite, wantErr: ErrInvalidToken},
		{name: "geoip token in a domain field", token: "geoip:cn", kind: KindSite, wantErr: ErrWrongKind},
		{name: "geosite token in an ip field", token: "geosite:cn", kind: KindIP, wantErr: ErrWrongKind},
		{name: "empty attribute", token: "geosite:cn@", kind: KindSite, wantErr: ErrInvalidToken},
		{
			name:  "a space inside the code stays part of the code",
			token: "geosite: cn",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: " cn"},
		},
		{
			name:  "a space inside an attribute stays part of the attribute",
			token: "geosite:cn@ ads",
			kind:  KindSite,
			want:  Reference{File: "geosite.dat", Code: "cn", Attributes: []string{" ads"}},
		},
		{name: "empty attribute between two others", token: "geosite:cn@@ads", kind: KindSite, wantErr: ErrInvalidToken},
		{
			name:  "double negation folds back to a plain match",
			token: "!!geoip:cn",
			kind:  KindIP,
			want:  Reference{File: "geoip.dat", Code: "cn"},
		},
		{
			name:  "negation on both sides of the prefix cancels out",
			token: "!geoip:!cn",
			kind:  KindIP,
			want:  Reference{File: "geoip.dat", Code: "cn"},
		},
		{name: "bare ext-ip prefix", token: "ext-ip:", kind: KindIP, wantErr: ErrInvalidToken},
		{
			name:  "an attribute suffix is part of the code for ip rules",
			token: "geoip:cn@x",
			kind:  KindIP,
			want:  Reference{File: "geoip.dat", Code: "cn@x"},
		},
		{name: "attribute only", token: "geosite:@ads", kind: KindSite, wantErr: ErrInvalidToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.token, tt.kind)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.File != tt.want.File || got.Code != tt.want.Code || got.Reverse != tt.want.Reverse {
				t.Errorf("reference = %+v, want %+v", got, tt.want)
			}
			if len(got.Attributes) != len(tt.want.Attributes) {
				t.Fatalf("attributes = %v, want %v", got.Attributes, tt.want.Attributes)
			}
			for i, attribute := range tt.want.Attributes {
				if got.Attributes[i] != attribute {
					t.Errorf("attribute %d = %q, want %q", i, got.Attributes[i], attribute)
				}
			}
		})
	}
}
