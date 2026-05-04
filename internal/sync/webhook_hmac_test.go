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
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyHMACSignature_table(t *testing.T) {
	const secret = "topsecret"
	body := []byte(`{"hello":"world"}`)
	good := sign(body, secret)

	cases := []struct {
		name         string
		secret       string
		header       string
		body         []byte
		errSubstring string
	}{
		{name: "valid signature", secret: secret, header: good, body: body},
		{name: "secret unset", secret: "", header: good, body: body, errSubstring: "hmac secret"},
		{name: "header missing", secret: secret, header: "", body: body, errSubstring: "missing"},
		{name: "wrong prefix", secret: secret, header: "sha1=" + strings.TrimPrefix(good, signaturePrefix), body: body, errSubstring: "invalid signature format"},
		{name: "non-hex digest", secret: secret, header: signaturePrefix + "notahex", body: body, errSubstring: "decode signature"},
		{name: "wrong secret", secret: "other", header: good, body: body, errSubstring: "hmac mismatch"},
		{name: "tampered body", secret: secret, header: good, body: []byte(`{"hello":"tampered"}`), errSubstring: "hmac mismatch"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyHMACSignature(tc.secret, tc.header, tc.body)
			if tc.errSubstring == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errSubstring)
			}
			if !strings.Contains(err.Error(), tc.errSubstring) {
				t.Fatalf("expected error containing %q, got %q", tc.errSubstring, err.Error())
			}
		})
	}
}

func TestWebhookHandler_HMACGate(t *testing.T) {
	const secret = "topsecret"
	payload := []byte(`{
      "repository": {"full_name": "acme/ops"},
      "head_commit": {"id": "sha1"},
      "commits": []
    }`)

	cases := []struct {
		name           string
		handlerSecret  string
		signatureHdr   string
		method         string
		wantStatusCode int
	}{
		{
			name:           "valid signature is accepted",
			handlerSecret:  secret,
			signatureHdr:   sign(payload, secret),
			method:         http.MethodPost,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "missing signature is rejected",
			handlerSecret:  secret,
			signatureHdr:   "",
			method:         http.MethodPost,
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "wrong signature is rejected",
			handlerSecret:  secret,
			signatureHdr:   sign(payload, "wrong-secret"),
			method:         http.MethodPost,
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "method other than POST is rejected before HMAC",
			handlerSecret:  secret,
			signatureHdr:   sign(payload, secret),
			method:         http.MethodGet,
			wantStatusCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &WebhookHandler{WebhookSecret: tc.handlerSecret}
			req := httptest.NewRequest(tc.method, WebhookPath, bytes.NewReader(payload))
			if tc.signatureHdr != "" {
				req.Header.Set(SignatureHeader, tc.signatureHdr)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatusCode {
				t.Fatalf("status: got %d, want %d (body=%q)", rec.Code, tc.wantStatusCode, rec.Body.String())
			}
		})
	}
}
