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

package git

import (
	"testing"
	"time"

	"github.com/go-logr/logr"

	"accord/internal/config"
)

func TestBatchWorker_Enqueue_lastWriteWinsPerPath(t *testing.T) {
	w := NewBatchWorker(config.InventoryControllerConfig{
		GitRepoURL:    "",
		BatchInterval: time.Hour,
	}, logr.Discard())
	w.Enqueue(ExportJob{Path: "inventory/applications/ns/a.yaml", Content: []byte("a")})
	w.Enqueue(ExportJob{Path: "inventory/applications/ns/a.yaml", Content: []byte("b")})
	w.mu.Lock()
	got := string(w.pending["inventory/applications/ns/a.yaml"].content)
	w.mu.Unlock()
	if got != "b" {
		t.Fatalf("expected last enqueue to win, got %q", got)
	}
}

func TestBatchWorker_EnqueueArchive_lastWins(t *testing.T) {
	w := NewBatchWorker(config.InventoryControllerConfig{
		GitRepoURL:    "",
		BatchInterval: time.Hour,
	}, logr.Discard())
	w.Enqueue(ExportJob{Path: "inventory/applications/ns/a.yaml", Content: []byte("b")})
	w.EnqueueArchive("inventory/applications/ns/a.yaml")
	w.mu.Lock()
	op := w.pending["inventory/applications/ns/a.yaml"]
	w.mu.Unlock()
	if op.kind != opArchive {
		t.Fatalf("expected archive to win, got kind %v", op.kind)
	}
}

func TestCommitSubject_format(t *testing.T) {
	s := commitSubject(3)
	want := "chore(inventory): sync 3 resources [skip ci]"
	if s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestCommitMessageForBatch_archiveOnly(t *testing.T) {
	batch := map[string]pendingExport{
		"inventory/applications/ns/my-app.yaml": {kind: opArchive},
	}
	paths := sortedKeysBatch(batch)
	sub, body := commitMessageForBatch(batch, paths)
	want := "feat(archive): move deleted resource my-app to archive [skip ci]"
	if sub != want {
		t.Fatalf("subject: got %q want %q", sub, want)
	}
	if body != "inventory/applications/ns/my-app.yaml" {
		t.Fatalf("body: got %q", body)
	}
}

func TestCommitMessageForBatch_mixed(t *testing.T) {
	batch := map[string]pendingExport{
		"inventory/applications/ns/b.yaml": {kind: opWrite, content: []byte("x")},
		"inventory/applications/ns/a.yaml": {kind: opArchive},
	}
	paths := sortedKeysBatch(batch)
	sub, _ := commitMessageForBatch(batch, paths)
	if want := "chore(inventory): sync 1 and archive 1 resources [skip ci]"; sub != want {
		t.Fatalf("got %q want %q", sub, want)
	}
	_ = paths
}
