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

type exportOpKind int

const (
	opWrite exportOpKind = iota
	opArchive
)

type pendingExport struct {
	kind    exportOpKind
	content []byte
}

// BatchWorker debounces export paths and periodically commits and pushes to Git.
type BatchWorker struct {
	cfg config.InventoryControllerConfig
	log logr.Logger

	mu      sync.Mutex
	pending map[string]pendingExport
}

// NewBatchWorker constructs a worker. Call mgr.Add(worker) so it runs under the manager lifecycle.
func NewBatchWorker(cfg config.InventoryControllerConfig, log logr.Logger) *BatchWorker {
	return &BatchWorker{
		cfg:     cfg,
		log:     log.WithName("git-batch-worker"),
		pending: make(map[string]pendingExport),
	}
}

// Enqueue records (or overwrites) a path for the next batch flush (debounce: last write wins).
func (w *BatchWorker) Enqueue(job ExportJob) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending[job.Path] = pendingExport{kind: opWrite, content: append([]byte(nil), job.Content...)}
}

// EnqueueArchive records a soft-delete: the live inventory file is moved to inventory/archive/
// with the same relative path on the next batch flush (debounce: last op wins per path).
func (w *BatchWorker) EnqueueArchive(relPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending[relPath] = pendingExport{kind: opArchive}
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
	w.pending = make(map[string]pendingExport)
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

func (w *BatchWorker) exportBatch(ctx context.Context, batch map[string]pendingExport) error {
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

	for rel, op := range batch {
		switch op.kind {
		case opWrite:
			full := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return fmt.Errorf("mkdir for %q: %w", rel, err)
			}
			if err := os.WriteFile(full, op.content, 0o644); err != nil {
				return fmt.Errorf("write export file %q: %w", rel, err)
			}
		case opArchive:
			if err := archiveInventoryFileInClone(dir, rel); err != nil {
				return fmt.Errorf("archive export file %q: %w", rel, err)
			}
		default:
			return fmt.Errorf("unknown export op for %q", rel)
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

	paths := sortedKeysBatch(batch)
	subject, body := commitMessageForBatch(batch, paths)
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

func sortedKeysBatch(m map[string]pendingExport) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func commitMessageForBatch(batch map[string]pendingExport, sortedPaths []string) (subject, body string) {
	var writes, archives []string
	for _, p := range sortedPaths {
		switch batch[p].kind {
		case opWrite:
			writes = append(writes, p)
		case opArchive:
			archives = append(archives, p)
		}
	}
	switch {
	case len(writes) > 0 && len(archives) == 0:
		return commitSubject(len(writes)), commitBody(writes)
	case len(writes) == 0 && len(archives) > 0:
		if len(archives) == 1 {
			name := resourceNameFromInventoryPath(archives[0])
			return fmt.Sprintf("feat(archive): move deleted resource %s to archive [skip ci]", name), commitBody(archives)
		}
		return fmt.Sprintf("feat(archive): move deleted resource %d resources to archive [skip ci]", len(archives)), commitBody(archives)
	default:
		subj := fmt.Sprintf("chore(inventory): sync %d and archive %d resources [skip ci]", len(writes), len(archives))
		b1 := commitBody(writes)
		b2 := commitBody(archives)
		switch {
		case b1 == "":
			return subj, "archive:\n" + b2
		case b2 == "":
			return subj, "write:\n" + b1
		default:
			return subj, "write:\n" + b1 + "\narchive:\n" + b2
		}
	}
}

func archiveInventoryFileInClone(repoRoot, inventoryRel string) error {
	destRel, err := ArchiveRelativePath(inventoryRel)
	if err != nil {
		return err
	}
	src := filepath.Join(repoRoot, filepath.FromSlash(inventoryRel))
	dest := filepath.Join(repoRoot, filepath.FromSlash(destRel))
	st, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			// No live file in the clone (e.g. never exported); nothing to move.
			return nil
		}
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("expected file at %q, got directory", inventoryRel)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir archive for %q: %w", destRel, err)
	}
	_ = os.Remove(dest)
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", src, dest, err)
	}
	return nil
}
