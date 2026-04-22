# 🛠️ Accord Project Worklog

> **Agent Instruction:** Append completed tasks here. Do NOT modify or delete past entries. Format: `[YYYY-MM-DD] Task Name - Result - Commit Hash`.

## 📜 Work History (Append-Only Log)
*이 섹션은 절대 지워지지 않으며, 위에서 완료된 작업이 아래로 누적됩니다.*
- **[2026-04-15] Phase 1 — Delete → archive (prompt `0006-phase1-archive`, architecture §7.4):** `Application` / `ApplicationSet` reconcilers treat `apierrors.IsNotFound` as a cluster delete, clear the material hash cache, render the inventory path, and call `git.BatchWorker.EnqueueArchive` (same `pending` map and ticker as `Enqueue`, last op per path wins; default batch interval 10s). The batch worker renames the live file to `inventory/archive/...` inside the clone instead of removing it. Pure single-archive batches use the commit subject `feat(archive): move deleted resource <name> to archive [skip ci]`; mixed write+archive batches use a `chore(inventory): sync … and archive …` subject. **Commit:** `0c85c78`.
- **[2026-04-15] Phase 1.5 — Sync operator (`docs/phase1-sync.md`):** Added `internal/sync` with GitHub push parsing (`commits[].added`/`modified`), strict `inventory/applications|applicationsets/...` path filter, raw content fetch from `raw.githubusercontent.com` using `GIT_ACCESS_TOKEN` (never logged), material hash via `inventory.MaterialHashFromNormalizedYAML`, inject `accord.io/sync-content-hash` before server-side apply (`accord-sync-operator` field owner). Registered `POST /api/v1/webhook`. Extended `inventory` normalization to strip sync hash from material digest so inventory loop-break matches. Removed legacy `internal/syncoperator` ConfigMap push handler. **Commit:** `abe43c7` (follow-up `bba1b0f` removes accidental `.DS_Store` files).
- **[2026-04-17] Phase 1 — Inventory controller & Git export (prompt 0003):** Implemented `internal/config` (`GIT_REPO_URL`, `GIT_BRANCH`, `BATCH_INTERVAL_SECONDS`, `EXPORT_PATH_TEMPLATE`, secrets without logging), refactored inventory reconcilers to watch `argoproj.io` `Application` and `ApplicationSet` as unstructured objects, hash/loop-break per `docs/phase1-inventory.md`, debounced Git batch worker in `internal/git/worker.go` with commit format `chore(inventory): sync <n> resources [skip ci]` and body (up to 10 paths). Removed ConfigMap MVP from inventory. Argo Go API dependency avoided (k8s 0.35 incompatibility); uses unstructured + `AddKnownTypes` instead. **Commit:** `e765208840e1a873a697516e4951ed8f5e66f44d`
- **[2026-04-15] Bootstrap & MVP Setup:** Initialized Kubebuilder project (`ops.accord.io/v1alpha1`). Split binaries (`inventory-controller`, `sync-operator`, `mirror-upgrader`). Created TDD normalization logic for ConfigMap.
  - **Commit:** `feat: run inventory reconcile and sync webhook in one manager` & `feat(inventory): add YAML normalization hash with TDD tests`
- **[2026-04-15] Architecture Correction:** Fixed the single `main.go` anomaly. Separated binaries according to the architecture documentation. Adopted `accord.io/sync-content-hash` for cross-component idempotency.
- **[2026-04-15] Task:** Bootstrap Kubebuilder & CRD Definition
  - **Result:** Scaffolding complete. `MirrorUpgradeRequest` struct created in `api/v1alpha1/`.
  - **Commit:** `feat: init kubebuilder and define MirrorUpgradeRequest CRD`
- **[2026-04-15] Task:** Implement YAML Normalization TDD
  - **Result:** Created `internal/inventory/normalize_test.go` and `normalize.go`. Hash validation tests passed.
  - **Commit:** `feat(inventory): add YAML normalization hash with TDD tests`
- **[2026-04-15] Issue/Mistake:** Agent mistakenly merged inventory and sync logic into a single `main.go`.
  - **Action:** Added User Override Rejection Protocol to `.cursorrules` to prevent future architectural violations.