package security

import (
	"strings"
	"testing"
)

func TestCompareToken(t *testing.T) {
	tests := []struct {
		name      string
		expected  string
		presented string
		want      bool
	}{
		{"exact match", "s3cret", "s3cret", true},
		{"wrong value", "s3cret", "s3cres", false},
		{"presented is a prefix", "s3cret", "s3c", false},
		{"presented is longer", "s3cret", "s3cretx", false},
		{"case differs", "s3cret", "S3CRET", false},
		{"empty expected never matches", "", "", false},
		{"empty expected rejects any credential", "", "anything", false},
		{"empty presented against real token", "s3cret", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareToken(tc.expected, tc.presented); got != tc.want {
				t.Errorf("CompareToken(%q, %q) = %v, want %v", tc.expected, tc.presented, got, tc.want)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{"standard", "Bearer abc123", "abc123", true},
		{"lowercase scheme", "bearer abc123", "abc123", true},
		{"mixed case scheme", "BeArEr abc123", "abc123", true},
		{"padded value", "Bearer   abc123  ", "abc123", true},
		{"empty header", "", "", false},
		{"no scheme", "abc123", "", false},
		{"wrong scheme", "Basic abc123", "", false},
		{"scheme only", "Bearer ", "", false},
		{"scheme with blanks", "Bearer    ", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, ok := BearerToken(tc.header)
			if ok != tc.wantOK || token != tc.wantToken {
				t.Errorf("BearerToken(%q) = (%q, %v), want (%q, %v)",
					tc.header, token, ok, tc.wantToken, tc.wantOK)
			}
		})
	}
}

func TestTokenFromEnv(t *testing.T) {
	const env = "SECSGEM_TEST_TOKEN"

	t.Run("flag wins", func(t *testing.T) {
		t.Setenv(env, "from-env")
		if got := TokenFromEnv("from-flag", env); got != "from-flag" {
			t.Errorf("got %q, want from-flag", got)
		}
	})

	t.Run("env fallback", func(t *testing.T) {
		t.Setenv(env, "from-env")
		if got := TokenFromEnv("", env); got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})

	t.Run("trailing newline trimmed", func(t *testing.T) {
		t.Setenv(env, "from-env\n")
		if got := TokenFromEnv("", env); got != "from-env" {
			t.Errorf("got %q, want from-env", got)
		}
	})

	t.Run("both empty", func(t *testing.T) {
		t.Setenv(env, "")
		if got := TokenFromEnv("   ", env); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestIsLoopbackListen(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"127.0.0.5:8080", true},
		{"localhost:8080", true},
		{"[::1]:8080", true},
		{":8080", false},
		{"0.0.0.0:8080", false},
		{"[::]:8080", false},
		{"192.168.1.10:8080", false},
		{"example.com:8080", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			if got := IsLoopbackListen(tc.addr); got != tc.want {
				t.Errorf("IsLoopbackListen(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

func TestRequireTokenForListen(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		addr    string
		wantErr bool
	}{
		{"public bind without token is refused", "", ":8080", true},
		{"wildcard v4 without token is refused", "", "0.0.0.0:8080", true},
		{"routable bind without token is refused", "", "192.168.1.10:8080", true},
		{"public bind with token is allowed", "s3cret", ":8080", false},
		{"loopback without token is allowed", "", "127.0.0.1:8080", false},
		{"loopback v6 without token is allowed", "", "[::1]:8080", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireTokenForListen(tc.token, tc.addr, EnvAPIToken)
			if (err != nil) != tc.wantErr {
				t.Fatalf("RequireTokenForListen(%q, %q) error = %v, wantErr %v",
					tc.token, tc.addr, err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), EnvAPIToken) {
				t.Errorf("error should name the env var, got: %v", err)
			}
		})
	}
}

func TestParseOriginsAndOriginAllowed(t *testing.T) {
	if got := ParseOrigins(""); got != nil {
		t.Errorf("empty list should parse to nil, got %v", got)
	}
	if got := ParseOrigins(" , ,"); got != nil {
		t.Errorf("blank entries should be dropped, got %v", got)
	}

	list := ParseOrigins("https://a.example.com, https://b.example.com")
	if len(list) != 2 {
		t.Fatalf("got %v, want 2 entries", list)
	}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"listed", "https://a.example.com", true},
		{"listed, different case", "HTTPS://A.EXAMPLE.COM", true},
		{"not listed", "https://evil.example.com", false},
		{"empty origin", "", false},
		{"wildcard is not special", "*", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := OriginAllowed(list, tc.origin); got != tc.want {
				t.Errorf("OriginAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}

	if OriginAllowed(nil, "https://a.example.com") {
		t.Error("an empty allowlist must deny every origin")
	}
}
