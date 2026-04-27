# 🔭 Phase 2: Release Watcher Specification

## 1. Component Overview
- **Binary:** `cmd/release-watcher/main.go`
- **Execution Model:** Designed to run as a Kubernetes `CronJob` (runs once and exits 0 on success). 
- **Goal:** Poll the official Argo CD GitHub repository for new releases, compare against existing upgrade requests, and generate a new `MirrorUpgradeRequest` CR if a new version is found.

## 2. Polling Logic (GitHub API)
- **Target URL:** `https://api.github.com/repos/argoproj/argo-cd/releases/latest`
- **Action:** Perform an HTTP GET request to fetch the latest release JSON.
- **Authentication:** Use `GIT_ACCESS_TOKEN` (via `Authorization: Bearer <token>`) to prevent rate-limiting.
- **Extraction:** Extract the `tag_name` (e.g., `v3.3.6`) and `body` (Changelog summary). Ignore drafts or pre-releases.

## 3. Idempotency & Validation (CRITICAL)
Before creating a new CR, the watcher MUST check if a request for this version already exists to avoid duplicate CRs.
- **Action:** List all existing `MirrorUpgradeRequest` resources in the cluster.
- **Condition:** If any CR exists where `spec.desiredVersion == <fetched_tag_name>`, log "Release already tracked" and exit gracefully (Exit 0).

## 4. CR Generation (The Proposal)
If the version is genuinely new, create a `MirrorUpgradeRequest` object.
- **API Group:** `ops.accord.io/v1alpha1`
- **Kind:** `MirrorUpgradeRequest`
- **Name:** Generate a safe Kubernetes name (e.g., `argocd-upgrade-v3-3-6`).
- **Spec:**
  - `targetCluster`: "mirror-argocd"
  - `targetNamespace`: "argocd"
  - `desiredVersion`: `<fetched_tag_name>`
  - `approvalMode`: "Manual"
- **Status (Optional but recommended to initialize):**
  - `phase`: "Pending"

## 5. Kubernetes Client Setup
Since this runs as a standalone script/Job rather than a continuous controller, instantiate a simple `controller-runtime` client or standard `client-go` client, perform the logic, and exit. Do NOT start a long-running manager with `.Start(ctx)`.