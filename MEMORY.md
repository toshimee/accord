# 🧠 Accord Project Memory & State

> **Agent Instruction:** Read this file at the start of every session to establish context. Update this file whenever a task is completed, a new bug is found, or an architectural decision is made.

## 📌 Current Status
- **Active Phase:** Phase 1 (Inventory Controller & Sync Operator setup)
- **Current Focus:** Iterating in a single `cmd/main.go` manager that runs both inventory-style reconcile (labeled `ConfigMap` watch + hash cache) and a sync webhook (`POST /accord/v1/sync/push`) sharing the same in-memory cache for loop breaking.

## 🏗️ Core Context (Do Not Forget)
- **Architecture:** 5 separate micro-components (Inventory, Sync, Watcher, Upgrader, Collector).
- **Key Mechanism:** Bidirectional sync requires SHA-256 Hash caching in the `inventory-controller` to break infinite loops.
- **Layout:** Standard Kubebuilder (`api/`, `cmd/`, `internal/`, `config/`).

## ✅ Task Backlog (Checklist)
- [x] Initialize Kubebuilder project with `ops.accord.io/v1alpha1`.
- [x] Define `MirrorUpgradeRequest` struct in `api/v1alpha1/`.
- [x] Run `make manifests` to generate CRD YAML (`config/crd/bases/ops.accord.io_mirrorupgraderequests.yaml`).
- [ ] (Optional later) Split binaries into `cmd/inventory-controller` and `cmd/sync-operator` for production isolation.
- [x] Prototype inventory + sync in `cmd/main.go` with canonical JSON + SHA-256 hash cache (labeled `ConfigMap` MVP; not yet `internal/inventory/`).

## 🐛 Known Issues / Blockers
- None. Hash cache is process-local only (lost on restart); Git export / Argo `Application` watches are not implemented yet.

## 📓 Recent Architectural Decisions (ADR Summary)
- [2026-04-15] Decided to use a Webhook-based `sync-operator` for Git -> Cluster deployments to avoid Argo CD Self-Heal race conditions, relying on Hash validation for idempotency.
- [2026-04-15] Temporarily colocated `inventory-controller` reconcile logic and `sync-operator` HTTP webhook in `cmd/main.go` to increase iteration speed before splitting binaries again.