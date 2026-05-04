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
	"reflect"
	"strings"
	"testing"
)

func TestParseGitHubPushPaths(t *testing.T) {
	body := []byte(`{
  "repository": { "full_name": "acme/ops" },
  "head_commit": { "id": "abc123" },
  "commits": [
    { "added": ["inventory/applications/ns/a.yaml"], "modified": [] },
    { "added": [], "modified": ["inventory/applications/ns/a.yaml"] }
  ]
}`)
	full, sha, addedModified, removed, err := ParseGitHubPushPaths(body)
	if err != nil {
		t.Fatal(err)
	}
	if full != "acme/ops" || sha != "abc123" {
		t.Fatalf("meta: %s %s", full, sha)
	}
	if len(addedModified) != 1 || addedModified[0] != "inventory/applications/ns/a.yaml" {
		t.Fatalf("addedModified: %#v", addedModified)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removed paths, got: %#v", removed)
	}
}

func TestParseGitHubPushPaths_usesAfterWhenHeadCommitMissing(t *testing.T) {
	body := []byte(`{
  "repository": { "full_name": "acme/ops" },
  "head_commit": null,
  "after": "def456",
  "commits": [ { "added": ["a.txt"], "modified": [] } ]
}`)
	full, sha, _, _, err := ParseGitHubPushPaths(body)
	if err != nil {
		t.Fatal(err)
	}
	if full != "acme/ops" || sha != "def456" {
		t.Fatalf("got %s %s", full, sha)
	}
}

// TestParseGitHubPushPaths_table covers the ADR-0012 contract: a single push
// payload may carry adds/modifies/removes across multiple commits, and the
// parser must (a) de-duplicate, (b) keep added & removed in separate slices,
// and (c) treat a path appearing in both as added (live state wins).
func TestParseGitHubPushPaths_table(t *testing.T) {
	type want struct {
		fullName      string
		sha           string
		addedModified []string
		removed       []string
		errSubstring  string
	}
	cases := []struct {
		name string
		body string
		want want
	}{
		{
			name: "removed only",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": {"id": "sha1"},
              "commits": [
                {"removed": ["inventory/applications/ns/x.yaml"]}
              ]
            }`,
			want: want{
				fullName:      "acme/ops",
				sha:           "sha1",
				addedModified: []string{},
				removed:       []string{"inventory/applications/ns/x.yaml"},
			},
		},
		{
			name: "removed deduplicated across commits and sorted",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": {"id": "sha1"},
              "commits": [
                {"removed": ["inventory/applications/ns/b.yaml", "inventory/applications/ns/a.yaml"]},
                {"removed": ["inventory/applications/ns/a.yaml"]}
              ]
            }`,
			want: want{
				fullName: "acme/ops",
				sha:      "sha1",
				addedModified: []string{},
				removed: []string{
					"inventory/applications/ns/a.yaml",
					"inventory/applications/ns/b.yaml",
				},
			},
		},
		{
			name: "added and removed mixed - added wins on collision",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": {"id": "sha1"},
              "commits": [
                {"removed": ["inventory/applications/ns/a.yaml"]},
                {"added":   ["inventory/applications/ns/a.yaml"]},
                {"removed": ["inventory/applications/ns/b.yaml"]}
              ]
            }`,
			want: want{
				fullName:      "acme/ops",
				sha:           "sha1",
				addedModified: []string{"inventory/applications/ns/a.yaml"},
				removed:       []string{"inventory/applications/ns/b.yaml"},
			},
		},
		{
			name: "modified counts as added (de-dup vs removed)",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": {"id": "sha1"},
              "commits": [
                {"removed":  ["inventory/applications/ns/c.yaml"]},
                {"modified": ["inventory/applications/ns/c.yaml"]}
              ]
            }`,
			want: want{
				fullName:      "acme/ops",
				sha:           "sha1",
				addedModified: []string{"inventory/applications/ns/c.yaml"},
				removed:       []string{},
			},
		},
		{
			name: "empty strings ignored",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": {"id": "sha1"},
              "commits": [
                {"added": [""], "modified": [""], "removed": [""]}
              ]
            }`,
			want: want{
				fullName:      "acme/ops",
				sha:           "sha1",
				addedModified: []string{},
				removed:       []string{},
			},
		},
		{
			name: "after fallback when head_commit missing",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": null,
              "after": "fallback-sha",
              "commits": [{"removed": ["inventory/applicationsets/ns/y.yaml"]}]
            }`,
			want: want{
				fullName:      "acme/ops",
				sha:           "fallback-sha",
				addedModified: []string{},
				removed:       []string{"inventory/applicationsets/ns/y.yaml"},
			},
		},
		{
			name: "missing repo full_name rejected",
			body: `{
              "repository": {"full_name": ""},
              "head_commit": {"id": "sha1"},
              "commits": []
            }`,
			want: want{errSubstring: "repository.full_name"},
		},
		{
			name: "missing commit SHA rejected",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": null,
              "after": "",
              "commits": []
            }`,
			want: want{errSubstring: "missing commit SHA"},
		},
		{
			name: "zero SHA rejected (branch deletion)",
			body: `{
              "repository": {"full_name": "acme/ops"},
              "head_commit": null,
              "after": "0000000000000000000000000000000000000000",
              "commits": []
            }`,
			want: want{errSubstring: "missing commit SHA"},
		},
		{
			name: "malformed json rejected",
			body: `not json`,
			want: want{errSubstring: "decode GitHub push JSON"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full, sha, addedModified, removed, err := ParseGitHubPushPaths([]byte(tc.body))
			if tc.want.errSubstring != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.want.errSubstring)
				}
				if !strings.Contains(err.Error(), tc.want.errSubstring) {
					t.Fatalf("expected error containing %q, got %q", tc.want.errSubstring, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if full != tc.want.fullName {
				t.Errorf("fullName: got %q, want %q", full, tc.want.fullName)
			}
			if sha != tc.want.sha {
				t.Errorf("sha: got %q, want %q", sha, tc.want.sha)
			}
			if !reflect.DeepEqual(addedModified, tc.want.addedModified) {
				t.Errorf("addedModified: got %#v, want %#v", addedModified, tc.want.addedModified)
			}
			if !reflect.DeepEqual(removed, tc.want.removed) {
				t.Errorf("removed: got %#v, want %#v", removed, tc.want.removed)
			}
		})
	}
}
