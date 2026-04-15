# 🧠 Accord Project Memory & State

> **Agent Instruction:** Read this file at the start of every session to establish context. Update this file whenever a task is completed, a new bug is found, or an architectural decision is made.

## 📌 Current Status
- **Active Phase:** Phase 1 (Inventory Controller & Sync Operator setup)
- **Current Focus:** Hardening per `.cursorrules`: separate binaries under `cmd/inventory-controller`, `cmd/sync-operator`, and `cmd/mirror-upgrader`; material hash + annotation contract in `internal/configmapmaterial`.

## 🏗️ Core Context (Do Not Forget)
- **Architecture:** 5 separate micro-components (Inventory, Sync, Watcher, Upgrader, Collector).
- **Key Mechanism:** Bidirectional sync requires SHA-256 Hash caching in the `inventory-controller` to break infinite loops.
- **Layout:** Standard Kubebuilder (`api/`, `cmd/`, `internal/`, `config/`).

## ✅ Task Backlog (Checklist)
- [x] Initialize Kubebuilder project with `ops.accord.io/v1alpha1`.
- [x] Define `MirrorUpgradeRequest` struct in `api/v1alpha1/`.
- [x] Run `make manifests` to generate CRD YAML (`config/crd/bases/ops.accord.io_mirrorupgraderequests.yaml`).
- [x] Split binaries: `cmd/inventory-controller`, `cmd/sync-operator`, `cmd/mirror-upgrader` (default deploy image entrypoint: mirror-upgrader).
- [x] ConfigMap material canonical JSON + SHA-256 in `internal/configmapmaterial` with table-driven tests; reconcile in `internal/inventory`; webhook in `internal/syncoperator`.
- [x] `internal/inventory/normalize.go` + `normalize_test.go`: YAML manifests strip `status`, volatile `metadata`, and `kubectl.kubernetes.io/last-applied-configuration`, then SHA-256 of canonical JSON (`MaterialHashFromNormalizedYAML`); tests assert noisy vs minimal YAML produce identical hashes.

## 🐛 Known Issues / Blockers
- None. Cross-component loop break uses `accord.io/sync-content-hash` (written by sync-operator) plus in-process cache in inventory-controller; Git export / Argo `Application` watches are not implemented yet.

## 📓 Recent Architectural Decisions (ADR Summary)
- [2026-04-15] Decided to use a Webhook-based `sync-operator` for Git -> Cluster deployments to avoid Argo CD Self-Heal race conditions, relying on Hash validation for idempotency.
- [2026-04-15] Reverted the experimental single-`main.go` merge: components must stay in separate `cmd/<component>/` entrypoints per `.cursorrules`; cross-pod idempotency uses `accord.io/sync-content-hash` instead of a shared in-memory cache between binaries.