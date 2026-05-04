/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// SignatureHeader is the HTTP header GitHub-style providers use to carry the
// HMAC-SHA256 signature of the raw push body (see ADR-0013).
const SignatureHeader = "X-Hub-Signature-256"

const signaturePrefix = "sha256="

// VerifyHMACSignature constant-time compares an X-Hub-Signature-256 header value
// (formatted as "sha256=<hex>") to HMAC-SHA256(body, secret).
//
// It returns a non-nil error for any failure mode (missing secret, missing
// header, malformed prefix, malformed hex, hash mismatch). Callers MUST treat
// every non-nil return as a 401 and SHOULD NOT echo the error detail back to
// the client (per docs/adr/0008-webhook-response-logging-strategy.md).
func VerifyHMACSignature(secret, headerValue string, body []byte) error {
	if secret == "" {
		return errors.New("hmac secret not configured")
	}
	if headerValue == "" {
		return fmt.Errorf("missing %s header", SignatureHeader)
	}
	if !strings.HasPrefix(headerValue, signaturePrefix) {
		return errors.New("invalid signature format")
	}
	received, err := hex.DecodeString(strings.TrimPrefix(headerValue, signaturePrefix))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(received, expected) {
		return errors.New("hmac mismatch")
	}
	return nil
}
