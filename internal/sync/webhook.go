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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// WebhookHandler handles GitHub-style push webhooks (docs/phase1-sync.md).
type WebhookHandler struct {
	K8s         client.Client
	HTTPClient  *http.Client
	GitHubToken string
}

type webhookResult struct {
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type webhookResponse struct {
	Results []webhookResult `json:"results"`
}

// ServeHTTP implements POST /api/v1/webhook for GitHub push payloads.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := logf.FromContext(r.Context())
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	fullName, sha, paths, err := ParseGitHubPushPaths(body)
	if err != nil {
		log.Info("Rejected webhook payload", "reason", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	httpClient := h.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	var results []webhookResult
	for _, p := range paths {
		if _, ok := ParseInventoryExportPath(p); !ok {
			results = append(results, webhookResult{Path: p, Status: "ignored", Detail: "path not under inventory/applications|applicationsets"})
			continue
		}
		raw, err := FetchGitHubRawFile(ctx, httpClient, h.GitHubToken, fullName, sha, p)
		if err != nil {
			results = append(results, webhookResult{Path: p, Status: "error", Detail: err.Error()})
			continue
		}
		if err := ApplyInventoryYAML(ctx, h.K8s, raw); err != nil {
			results = append(results, webhookResult{Path: p, Status: "error", Detail: err.Error()})
			continue
		}
		results = append(results, webhookResult{Path: p, Status: "applied"})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	if err := enc.Encode(webhookResponse{Results: results}); err != nil {
		log.Error(err, "Failed to encode webhook response")
	}
}
