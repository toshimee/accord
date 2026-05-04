# 🧠 Accord Project Memory & State

> **Agent Instruction:** Read this file first. Remove items from "Active Tasks" as you complete them, and log your work in `WORKLOG.md`.

## 📌 Current Status
- **Active Phase:** Phase 1 / 1.5 (Inventory + Sync)
- **Current Focus:** Phase 2 (release-watcher) preparation. P0 hotfixes from review 0001 are landed (1.1–1.4). Next P1: Resurrection unarchive (review 1.6), Server-side dry-run (1.5), Git push rebase (1.7), flush-merge ordering (1.10).
- **P0 hotfix sprint applied (prompt `0009-phase1-claude-review-applied`):**
  - **1.1** `internal/sync/github_test.go` realigned to ADR-0012 5-value signature; table-driven tests for `removed` semantics. Commit `908633b` (with fix-up `cf39f78` promoting the previously-uncommitted ADR-0012 hunks in `github.go` + `webhook.go`).
  - **1.2** Webhook HMAC: `internal/config.SyncOperatorConfig{WebhookSecret, GitAccessToken}` (fail-closed on empty), `internal/sync/hmac.go::VerifyHMACSignature` (constant-time), handler 401 gate before `ParseGitHubPushPaths`. ADR-0013 (untracked locally per `.git/info/exclude`). Commit `fed6beb`.
  - **1.3** Removed-path validation unified on `ParseInventoryExportPath` + `ArgoKindFromPlural`; invalid paths now surface as `ignored` results, `IsNotFound` returns `deleted: already gone`. Commit `a33d12d`.
  - **1.4** `ApplyInventoryYAML(ctx, c, parsed, gitYAML)` enforces apiVersion/kind/namespace/name match against the parsed file path; mismatches return typed `*PathManifestMismatchError` and skip SSA. Commit `c7afa04`.
- **Webhook secret env var:** `WEBHOOK_SECRET` (provider-agnostic). `GITHUB_WEBHOOK_SECRET` was rejected to keep room for GitLab/Bitbucket. `sync-operator` will CrashLoopBackOff if unset.
- **Phase 1 archive (§7.4):** On `apierrors.IsNotFound`, inventory-controller enqueues `git.BatchWorker.EnqueueArchive` (same debounced map as exports). The batch worker **renames** `inventory/.../name.yaml` → `inventory/archive/.../name.yaml` in the clone (soft-delete; no hard remove). Default `BATCH_INTERVAL_SECONDS` is **10s** when unset.

## 🏗️ Core Context (Do Not Forget)
- **Architecture:** 5 separate micro-components (Inventory, Sync, Watcher, Upgrader, Collector).
- **Key Mechanism:** Bidirectional sync requires SHA-256 Hash caching in the `inventory-controller` to break infinite loops.
- **Layout:** Standard Kubebuilder (`api/`, `cmd/`, `internal/`, `config/`).

## ✅ Task Backlog (Checklist)
- [x] Initialize Kubebuilder project with `ops.accord.io/v1alpha1`.
- [x] Define `MirrorUpgradeRequest` struct in `api/v1alpha1/`.
- [x] Run `make manifests` to generate CRD YAML (`config/crd/bases/ops.accord.io_mirrorupgraderequests.yaml`).
- [x] Split binaries: `cmd/inventory-controller`, `cmd/sync-operator`, `cmd/mirror-upgrader` (default deploy image entrypoint: mirror-upgrader).
- [x] ConfigMap material canonical JSON + SHA-256 in `internal/configmapmaterial` with table-driven tests (legacy ConfigMap path removed from sync-operator).
- [x] Phase 1.5 sync-operator: `internal/sync` GitHub push webhook `POST /api/v1/webhook`, path filter `inventory/applications|applicationsets/...`, raw.githubusercontent.com fetch, `MaterialHashFromNormalizedYAML` + inject `accord.io/sync-content-hash` then SSA (`accord-sync-operator` field manager). No shared inventory cache. Git-driven deletion sync handles `commits[].removed` and calls `Delete()` ignoring `IsNotFound`.
- [x] `internal/inventory/normalize.go` + `normalize_test.go`: YAML manifests strip `status`, volatile `metadata`, and `kubectl.kubernetes.io/last-applied-configuration`, then SHA-256 of canonical JSON (`MaterialHashFromNormalizedYAML`); tests assert noisy vs minimal YAML produce identical hashes.
- [x] Phase 1 inventory: `internal/config` (12-factor env), Argo `Application` / `ApplicationSet` watches via `unstructured` + `internal/git` batch worker (`chore(inventory): sync N resources [skip ci]`), export path `inventory/<plural>/...`, loop break via annotation + cache.
- [x] Phase 1 — resource delete → Git **archive** (prompt 0006): `IsNotFound` → `EnqueueArchive`; `internal/git` moves live YAML under `inventory/archive/...` with `feat(archive): move deleted resource <name> to archive [skip ci]` when the batch is archive-only (single resource).

## 🐛 Known Issues / Blockers
- Git batch worker uses a **fresh shallow clone per flush**; `docs/git-policy.md` “pull --rebase before push” is not yet implemented in-process (go-git `PullOptions` has no rebase flag in the pinned version). Non-fast-forward pushes fail and paths are re-queued.

## 📓 Recent Architectural Decisions (ADR Summary)
- [2026-05-04] **ADR-0013** Webhook HMAC validation with `WEBHOOK_SECRET`. Fail-closed startup; `X-Hub-Signature-256` constant-time compare via `hmac.Equal`; handler returns 401 with no body (per ADR-0008). Provider-agnostic env var name.
- [2026-04-15] Decided to use a Webhook-based `sync-operator` for Git -> Cluster deployments to avoid Argo CD Self-Heal race conditions, relying on Hash validation for idempotency.
- [2026-04-15] Reverted the experimental single-`main.go` merge: components must stay in separate `cmd/<component>/` entrypoints per `.cursorrules`; cross-pod idempotency uses `accord.io/sync-content-hash` instead of a shared in-memory cache between binaries.
- [2026-04-15] Inventory watches Argo CRDs as `unstructured.Unstructured` (scheme `AddKnownTypes`) instead of importing `github.com/argoproj/argo-cd/v2` APIs, which pulled `gitops-engine` / `k8s.io/kubernetes` and broke builds against `k8s.io/*` v0.35.
