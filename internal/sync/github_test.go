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

import "testing"

func TestParseGitHubPushPaths(t *testing.T) {
	body := []byte(`{
  "repository": { "full_name": "acme/ops" },
  "head_commit": { "id": "abc123" },
  "commits": [
    { "added": ["inventory/applications/ns/a.yaml"], "modified": [] },
    { "added": [], "modified": ["inventory/applications/ns/a.yaml"] }
  ]
}`)
	full, sha, paths, err := ParseGitHubPushPaths(body)
	if err != nil {
		t.Fatal(err)
	}
	if full != "acme/ops" || sha != "abc123" {
		t.Fatalf("meta: %s %s", full, sha)
	}
	if len(paths) != 1 || paths[0] != "inventory/applications/ns/a.yaml" {
		t.Fatalf("paths: %#v", paths)
	}
}

func TestParseGitHubPushPaths_usesAfterWhenHeadCommitMissing(t *testing.T) {
	body := []byte(`{
  "repository": { "full_name": "acme/ops" },
  "head_commit": null,
  "after": "def456",
  "commits": [ { "added": ["a.txt"], "modified": [] } ]
}`)
	full, sha, _, err := ParseGitHubPushPaths(body)
	if err != nil {
		t.Fatal(err)
	}
	if full != "acme/ops" || sha != "def456" {
		t.Fatalf("got %s %s", full, sha)
	}
}
