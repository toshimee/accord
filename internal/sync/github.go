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
	"encoding/json"
	"fmt"
	"sort"
)

// githubPushEvent is a minimal subset of GitHub's push webhook JSON.
// See: https://docs.github.com/en/webhooks/webhook-events-and-payloads#push
type githubPushEvent struct {
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	HeadCommit *struct {
		ID string `json:"id"`
	} `json:"head_commit"`
	// After is the new repository head SHA; present even when head_commit is omitted.
	After   string `json:"after"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

// ParseGitHubPushPaths returns unique file paths from commits[].added/modified and commits[].removed.
func ParseGitHubPushPaths(body []byte) (fullName, headSHA string, addedModified []string, removed []string, err error) {
	var ev githubPushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return "", "", nil, nil, fmt.Errorf("decode GitHub push JSON: %w", err)
	}
	if ev.Repository.FullName == "" {
		return "", "", nil, nil, fmt.Errorf("missing repository.full_name")
	}
	sha := ""
	if ev.HeadCommit != nil {
		sha = ev.HeadCommit.ID
	}
	if sha == "" {
		sha = ev.After
	}
	if sha == "" || sha == "0000000000000000000000000000000000000000" {
		return "", "", nil, nil, fmt.Errorf("missing commit SHA (head_commit.id or after)")
	}
	seenAdded := make(map[string]struct{})
	seenRemoved := make(map[string]struct{})
	for _, c := range ev.Commits {
		for _, p := range c.Added {
			if p != "" {
				seenAdded[p] = struct{}{}
			}
		}
		for _, p := range c.Modified {
			if p != "" {
				seenAdded[p] = struct{}{}
			}
		}
		for _, p := range c.Removed {
			if p != "" {
				seenRemoved[p] = struct{}{}
			}
		}
	}
	
	// added와 removed에 동시 존재하면 added로 간주
	for p := range seenAdded {
		delete(seenRemoved, p)
	}

	outAdded := make([]string, 0, len(seenAdded))
	for p := range seenAdded {
		outAdded = append(outAdded, p)
	}
	sort.Strings(outAdded)

	outRemoved := make([]string, 0, len(seenRemoved))
	for p := range seenRemoved {
		outRemoved = append(outRemoved, p)
	}
	sort.Strings(outRemoved)

	return ev.Repository.FullName, sha, outAdded, outRemoved, nil
}
