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

package config

import (
	"testing"
	"time"
)

func TestLoadInventoryControllerConfig_defaults(t *testing.T) {
	t.Setenv(envGitRepoURL, "")
	t.Setenv(envGitBranch, "")
	t.Setenv(envBatchIntervalSeconds, "")
	t.Setenv(envExportPathTemplate, "")
	t.Setenv(envGitUsername, "")
	t.Setenv(envGitAccessToken, "")

	c, err := LoadInventoryControllerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.GitBranch != defaultGitBranch {
		t.Fatalf("GitBranch: got %q", c.GitBranch)
	}
	if c.BatchInterval != defaultBatchIntervalSec*time.Second {
		t.Fatalf("BatchInterval: got %v", c.BatchInterval)
	}
	if c.ExportPathTemplate != defaultExportPathTemplate {
		t.Fatalf("ExportPathTemplate: got %q", c.ExportPathTemplate)
	}
}

func TestLoadInventoryControllerConfig_batchInterval_invalid(t *testing.T) {
	t.Setenv(envBatchIntervalSeconds, "not-an-int")
	_, err := LoadInventoryControllerConfig()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadSyncOperatorConfig_table(t *testing.T) {
	cases := []struct {
		name         string
		secret       string
		token        string
		wantErr      bool
		wantSecret   string
		wantHasToken bool
	}{
		{name: "secret set, token set", secret: "shh", token: "ghp_x", wantSecret: "shh", wantHasToken: true},
		{name: "secret set, token empty", secret: "shh", token: "", wantSecret: "shh", wantHasToken: false},
		{name: "secret unset rejected", secret: "", token: "ghp_x", wantErr: true},
		{name: "secret whitespace rejected", secret: "   ", token: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envWebhookSecret, tc.secret)
			t.Setenv(envGitAccessToken, tc.token)
			c, err := LoadSyncOperatorConfig()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got config=%+v", c)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.WebhookSecret != tc.wantSecret {
				t.Errorf("WebhookSecret: got %q, want %q", c.WebhookSecret, tc.wantSecret)
			}
			if (c.GitAccessToken != "") != tc.wantHasToken {
				t.Errorf("GitAccessToken presence: got %t, want %t", c.GitAccessToken != "", tc.wantHasToken)
			}
		})
	}
}
