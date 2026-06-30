package elevenlabs

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	speechEngineJWTClockSkew = 60 * time.Second
	speechEngineJWTIssuer    = "https://api.elevenlabs.io/convai/speech-engine"
	speechEngineJWTSubject   = "convai_speech_engine_upstream"
)

type speechEngineJWTClaims struct {
	Exp json.Number `json:"exp"`
	Iat json.Number `json:"iat"`
	Iss string      `json:"iss"`
	Sub string      `json:"sub"`
}

type speechEngineJWTHeader struct {
	Alg string `json:"alg"`
}

// VerifySpeechEngineAuthorization verifies a Speech Engine upstream
// authorization JWT from the X-Elevenlabs-Speech-Engine-Authorization header.
func VerifySpeechEngineAuthorization(token, apiKey string, now time.Time) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("elevenlabs: speech engine authorization token is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("elevenlabs: api key is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("elevenlabs: malformed speech engine authorization token")
	}

	headerData, err := decodeSpeechEngineJWTPart(parts[0])
	if err != nil {
		return fmt.Errorf("elevenlabs: decode speech engine authorization header: %w", err)
	}
	var header speechEngineJWTHeader
	if err := decodeSpeechEngineJWTJSON(headerData, &header); err != nil {
		return fmt.Errorf("elevenlabs: parse speech engine authorization header: %w", err)
	}
	if header.Alg != "HS256" {
		return fmt.Errorf("elevenlabs: unsupported speech engine authorization algorithm %q", header.Alg)
	}

	payloadData, err := decodeSpeechEngineJWTPart(parts[1])
	if err != nil {
		return fmt.Errorf("elevenlabs: decode speech engine authorization claims: %w", err)
	}
	var claims speechEngineJWTClaims
	if err := decodeSpeechEngineJWTJSON(payloadData, &claims); err != nil {
		return fmt.Errorf("elevenlabs: parse speech engine authorization claims: %w", err)
	}

	signature, err := decodeSpeechEngineJWTPart(parts[2])
	if err != nil {
		return fmt.Errorf("elevenlabs: decode speech engine authorization signature: %w", err)
	}
	if !validSpeechEngineJWTSignature(parts[0]+"."+parts[1], signature, apiKey) {
		return errors.New("elevenlabs: invalid speech engine authorization signature")
	}

	if claims.Iss != speechEngineJWTIssuer {
		return errors.New("elevenlabs: invalid speech engine authorization issuer")
	}
	if claims.Sub != speechEngineJWTSubject {
		return errors.New("elevenlabs: invalid speech engine authorization subject")
	}
	if claims.Exp == "" {
		return errors.New("elevenlabs: speech engine authorization expiry is required")
	}
	expUnix, err := claims.Exp.Int64()
	if err != nil {
		return fmt.Errorf("elevenlabs: parse speech engine authorization expiry: %w", err)
	}
	if now.After(time.Unix(expUnix, 0).Add(speechEngineJWTClockSkew)) {
		return errors.New("elevenlabs: speech engine authorization token has expired")
	}
	if claims.Iat == "" {
		return errors.New("elevenlabs: speech engine authorization issued-at is required")
	}
	iatUnix, err := claims.Iat.Int64()
	if err != nil {
		return fmt.Errorf("elevenlabs: parse speech engine authorization issued-at: %w", err)
	}
	if time.Unix(iatUnix, 0).After(now.Add(speechEngineJWTClockSkew)) {
		return errors.New("elevenlabs: speech engine authorization issued-at is in the future")
	}

	return nil
}

func decodeSpeechEngineJWTJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func decodeSpeechEngineJWTPart(part string) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(part)
	if err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(part)
}

func validSpeechEngineJWTSignature(signingInput string, signature []byte, apiKey string) bool {
	secret := sha256.Sum256([]byte(apiKey))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(signingInput))
	return hmac.Equal(signature, mac.Sum(nil))
}
