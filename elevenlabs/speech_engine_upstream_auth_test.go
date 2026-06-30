package elevenlabs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVerifySpeechEngineAuthorizationAcceptsValidToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	token := speechEngineTestJWT(t, "test-key", map[string]any{
		"alg": "HS256",
	}, map[string]any{
		"exp": now.Add(time.Minute).Unix(),
		"iat": now.Add(-time.Minute).Unix(),
		"iss": speechEngineJWTIssuer,
		"sub": speechEngineJWTSubject,
	})

	if err := VerifySpeechEngineAuthorization(token, "test-key", now); err != nil {
		t.Fatalf("VerifySpeechEngineAuthorization returned error: %v", err)
	}
}

func TestVerifySpeechEngineAuthorizationRejectsInvalidTokens(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	tests := []struct {
		apiKey  string
		name    string
		token   string
		wantErr string
	}{
		{
			apiKey:  "test-key",
			name:    "missing token",
			token:   "",
			wantErr: "token is required",
		},
		{
			apiKey:  "",
			name:    "missing api key",
			token:   "a.b.c",
			wantErr: "api key is required",
		},
		{
			apiKey:  "test-key",
			name:    "malformed token",
			token:   "a.b",
			wantErr: "malformed",
		},
		{
			apiKey: "test-key",
			name:   "wrong algorithm",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "none",
			}, map[string]any{
				"exp": now.Add(time.Minute).Unix(),
				"iat": now.Add(-time.Minute).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "unsupported",
		},
		{
			apiKey: "wrong-key",
			name:   "bad signature",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"exp": now.Add(time.Minute).Unix(),
				"iat": now.Add(-time.Minute).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "signature",
		},
		{
			apiKey: "test-key",
			name:   "wrong issuer",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"exp": now.Add(time.Minute).Unix(),
				"iat": now.Add(-time.Minute).Unix(),
				"iss": "https://example.com",
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "issuer",
		},
		{
			apiKey: "test-key",
			name:   "wrong subject",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"exp": now.Add(time.Minute).Unix(),
				"iat": now.Add(-time.Minute).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": "wrong-subject",
			}),
			wantErr: "subject",
		},
		{
			apiKey: "test-key",
			name:   "missing expiry",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"iat": now.Add(-time.Minute).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "expiry is required",
		},
		{
			apiKey: "test-key",
			name:   "expired outside clock skew",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"exp": now.Add(-61 * time.Second).Unix(),
				"iat": now.Add(-time.Minute).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "expired",
		},
		{
			apiKey: "test-key",
			name:   "missing issued at",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"exp": now.Add(time.Minute).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "issued-at is required",
		},
		{
			apiKey: "test-key",
			name:   "issued at too far in future",
			token: speechEngineTestJWT(t, "test-key", map[string]any{
				"alg": "HS256",
			}, map[string]any{
				"exp": now.Add(time.Minute).Unix(),
				"iat": now.Add(61 * time.Second).Unix(),
				"iss": speechEngineJWTIssuer,
				"sub": speechEngineJWTSubject,
			}),
			wantErr: "issued-at is in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySpeechEngineAuthorization(tt.token, tt.apiKey, now)
			if err == nil {
				t.Fatal("VerifySpeechEngineAuthorization returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestVerifySpeechEngineAuthorizationAllowsClockSkew(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	token := speechEngineTestJWT(t, "test-key", map[string]any{
		"alg": "HS256",
	}, map[string]any{
		"exp": now.Add(-60 * time.Second).Unix(),
		"iat": now.Add(60 * time.Second).Unix(),
		"iss": speechEngineJWTIssuer,
		"sub": speechEngineJWTSubject,
	})

	if err := VerifySpeechEngineAuthorization(token, "test-key", now); err != nil {
		t.Fatalf("VerifySpeechEngineAuthorization returned error: %v", err)
	}
}

func speechEngineTestJWT(t *testing.T, apiKey string, header, payload map[string]any) string {
	t.Helper()

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := encodedHeader + "." + encodedPayload

	secret := sha256.Sum256([]byte(apiKey))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature
}
