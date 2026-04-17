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
	got := string(w.pending["inventory/applications/ns/a.yaml"])
	w.mu.Unlock()
	if got != "b" {
		t.Fatalf("expected last enqueue to win, got %q", got)
	}
}

func TestCommitSubject_format(t *testing.T) {
	s := commitSubject(3)
	want := "chore(inventory): sync 3 resources [skip ci]"
	if s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}
