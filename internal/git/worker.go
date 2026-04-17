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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-logr/logr"

	"accord/internal/config"
)

// ExportJob is one file to write under the Git repository root.
type ExportJob struct {
	Path    string
	Content []byte
}

// BatchWorker debounces export paths and periodically commits and pushes to Git.
type BatchWorker struct {
	cfg config.InventoryControllerConfig
	log logr.Logger

	mu      sync.Mutex
	pending map[string][]byte
}

// NewBatchWorker constructs a worker. Call mgr.Add(worker) so it runs under the manager lifecycle.
func NewBatchWorker(cfg config.InventoryControllerConfig, log logr.Logger) *BatchWorker {
	return &BatchWorker{
		cfg:     cfg,
		log:     log.WithName("git-batch-worker"),
		pending: make(map[string][]byte),
	}
}

// Enqueue records (or overwrites) a path for the next batch flush (debounce: last write wins).
func (w *BatchWorker) Enqueue(job ExportJob) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending[job.Path] = append([]byte(nil), job.Content...)
}

// NeedLeaderElection ensures only the elected inventory-controller replica pushes to Git.
func (w *BatchWorker) NeedLeaderElection() bool {
	return true
}

// Start runs the batch ticker until ctx is cancelled.
func (w *BatchWorker) Start(ctx context.Context) error {
	t := time.NewTicker(w.cfg.BatchInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			w.flush(ctx)
		}
	}
}

func (w *BatchWorker) flush(ctx context.Context) {
	w.mu.Lock()
	if len(w.pending) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.pending
	w.pending = make(map[string][]byte)
	w.mu.Unlock()

	if strings.TrimSpace(w.cfg.GitRepoURL) == "" {
		w.log.Info("Skipping Git export batch because GIT_REPO_URL is empty",
			"droppedExports", len(batch))
		return
	}

	if err := w.exportBatch(ctx, batch); err != nil {
		w.log.Error(err, "Git export batch failed; paths will be retried on next flush")
		w.mu.Lock()
		for p, c := range batch {
			w.pending[p] = c
		}
		w.mu.Unlock()
	}
}

func (w *BatchWorker) auth() *http.BasicAuth {
	if w.cfg.GitAccessToken == "" {
		return nil
	}
	user := w.cfg.GitUsername
	if user == "" {
		user = "git"
	}
	return &http.BasicAuth{
		Username: user,
		Password: w.cfg.GitAccessToken,
	}
}

func (w *BatchWorker) exportBatch(ctx context.Context, batch map[string][]byte) error {
	dir, err := os.MkdirTemp("", "accord-inventory-git-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	branch := w.cfg.GitBranch
	if branch == "" {
		branch = "main"
	}

	cloneOpts := &git.CloneOptions{
		URL:           w.cfg.GitRepoURL,
		Depth:         1,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          w.auth(),
	}
	if _, err := git.PlainCloneContext(ctx, dir, false, cloneOpts); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	for rel, content := range batch {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("mkdir for %q: %w", rel, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return fmt.Errorf("write export file %q: %w", rel, err)
		}
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("git open: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git worktree: %w", err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if status.IsClean() {
		w.log.V(1).Info("Git export batch produced no diff; skipping commit")
		return nil
	}

	paths := sortedKeys(batch)
	subject := commitSubject(len(paths))
	body := commitBody(paths)
	fullMsg := subject
	if body != "" {
		fullMsg = subject + "\n\n" + body
	}
	if _, err := wt.Commit(fullMsg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  "accord-inventory",
			Email: "accord@local",
			When:  time.Now(),
		},
	}); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	if err := repo.PushContext(ctx, &git.PushOptions{Auth: w.auth()}); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	w.log.Info("Published inventory Git batch", "files", len(paths))
	return nil
}

func commitSubject(n int) string {
	return fmt.Sprintf("chore(inventory): sync %d resources [skip ci]", n)
}

func commitBody(paths []string) string {
	var b strings.Builder
	max := 10
	for i, p := range paths {
		if i >= max {
			fmt.Fprintf(&b, "... and %d more\n", len(paths)-max)
			break
		}
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
