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
	"fmt"
	"io"
	"net/http"
	"strings"
)

// FetchGitHubRawFile downloads file contents from raw.githubusercontent.com for a commit SHA.
// token must not be logged by callers (GitHub PAT or fine-grained token with contents:read).
func FetchGitHubRawFile(ctx context.Context, httpClient *http.Client, token, fullName, sha, filePath string) ([]byte, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	p := strings.TrimPrefix(strings.TrimSpace(filePath), "/")
	u := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", fullName, sha, p)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build raw request: %w", err)
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raw fetch status %d for %s", resp.StatusCode, u)
	}
	return body, nil
}
