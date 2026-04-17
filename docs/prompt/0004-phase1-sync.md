[Agent Instruction: Execute Phase 1.5 - Sync Operator]

We have successfully verified the Git Export pipeline. Now, it's time to build the other half of the bidirectional sync: the sync-operator.
Before writing any code, you MUST thoroughly read docs/phase1-sync.md and remember the architecture rules.

Your Tasks:

Webhook Server: Implement cmd/sync-operator/main.go and the core logic in internal/sync/. Create an HTTP endpoint (e.g., /api/v1/webhook) to receive Git Push payloads.

Path Filtering: Parse the incoming payload and strictly filter for files matching inventory/applications/<namespace>/<name>.yaml or inventory/applicationsets/.... Ignore everything else.

Deploy & Loop Break (CRITICAL): For valid files, compute the SHA-256 hash using the existing internal/inventory/normalize.go logic. You MUST inject the accord.io/sync-content-hash annotation with this hash value into the parsed Unstructured object BEFORE applying it to the cluster (Server-Side Apply preferred).

No Shared Cache: Remember that sync-operator is a separate binary. Do NOT attempt to access the inventory-controller's in-memory cache. Rely purely on the annotation for loop breaking.

Constraints & Memory:

Follow standard Go project layout and .cursorrules.

Do NOT use github.com/argoproj/argo-cd/v2 APIs to avoid k8s version conflicts. Parse the Git YAMLs into unstructured.Unstructured objects.

Update MEMORY.md to check off completed tasks and append your detailed commit log to WORKLOG.md.

Acknowledge you understand the Loop Break annotation mechanism, and begin implementation.