# 🧠 Accord Project Memory & State

> **Agent Instruction:** Read this file first. Remove items from "Active Tasks" as you complete them, and log your work in `WORKLOG.md`.

## 📌 Current Status
- **Active Phase:** Phase 1 / 1.5 (Inventory + Sync)
- **Current Focus:** Operational hardening (webhook auth, GitLab payloads, SSA conflict policy); optional ClusterRole split per component.

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
- [x] Phase 1.5 sync-operator: `internal/sync` GitHub push webhook `POST /api/v1/webhook`, path filter `inventory/applications|applicationsets/...`, raw.githubusercontent.com fetch, `MaterialHashFromNormalizedYAML` + inject `accord.io/sync-content-hash` then SSA (`accord-sync-operator` field manager). No shared inventory cache.
- [x] `internal/inventory/normalize.go` + `normalize_test.go`: YAML manifests strip `status`, volatile `metadata`, and `kubectl.kubernetes.io/last-applied-configuration`, then SHA-256 of canonical JSON (`MaterialHashFromNormalizedYAML`); tests assert noisy vs minimal YAML produce identical hashes.
- [x] Phase 1 inventory: `internal/config` (12-factor env), Argo `Application` / `ApplicationSet` watches via `unstructured` + `internal/git` batch worker (`chore(inventory): sync N resources [skip ci]`), export path `inventory/<plural>/...`, loop break via annotation + cache.

## 🐛 Known Issues / Blockers
- Git batch worker uses a **fresh shallow clone per flush**; `docs/git-policy.md` “pull --rebase before push” is not yet implemented in-process (go-git `PullOptions` has no rebase flag in the pinned version). Non-fast-forward pushes fail and paths are re-queued.

## 📓 Recent Architectural Decisions (ADR Summary)
- [2026-04-15] Decided to use a Webhook-based `sync-operator` for Git -> Cluster deployments to avoid Argo CD Self-Heal race conditions, relying on Hash validation for idempotency.
- [2026-04-15] Reverted the experimental single-`main.go` merge: components must stay in separate `cmd/<component>/` entrypoints per `.cursorrules`; cross-pod idempotency uses `accord.io/sync-content-hash` instead of a shared in-memory cache between binaries.
- [2026-04-15] Inventory watches Argo CRDs as `unstructured.Unstructured` (scheme `AddKnownTypes`) instead of importing `github.com/argoproj/argo-cd/v2` APIs, which pulled `gitops-engine` / `k8s.io/kubernetes` and broke builds against `k8s.io/*` v0.35.
