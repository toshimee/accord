# 🧩 Phase 1.5: Sync Operator Specification

## 1. Component Overview
- **Binary:** `cmd/sync-operator/main.go`
- **Goal:** Receive Git Push Webhooks, filter modified Argo CD manifests, and apply them to the cluster safely.
- **Role in Loop Break:** This component MUST inject a specific hash annotation before applying, so `inventory-controller` knows to ignore the resulting cluster event.

## 2. Webhook Payload Processing
1. **Endpoint:** Create an HTTP server listening on a specific path (e.g., `/api/v1/webhook`).
2. **Git Provider Support:** Initially support standard JSON payloads (e.g., GitHub Push Event).
3. **Extract Changed Files:** Parse the payload to extract files listed in `commits[].added` and `commits[].modified`.

## 3. Filtering & Path Validation
Do NOT blindly apply every changed file.
- **Rule:** Only process files that match the exact export path logic:
  `inventory/applications/<namespace>/<name>.yaml` OR `inventory/applicationsets/<namespace>/<name>.yaml`.
- **Ignore:** Any modifications to `docs/`, `config/`, or files outside the `inventory/` directory must be ignored.

## 4. Deploy & Loop Break Logic (CRITICAL)
When a valid YAML file is identified, the Sync Operator MUST follow this logic to prevent Bidirectional Sync infinite loops.

```go
func processFile(gitFileContent []byte) error {
    // 1. Calculate the Canonical Hash of the incoming Git file
    // Reuse the exact same logic from `internal/inventory/normalize.go`
    incomingHash := ComputeSHA256(gitFileContent)

    // 2. Parse the YAML into an Unstructured object
    obj := ParseToUnstructured(gitFileContent)

    // 3. Inject Loop-Break Annotation (CRITICAL)
    // The inventory-controller relies on this annotation to ignore the echo event.
    annotations := obj.GetAnnotations()
    if annotations == nil { annotations = make(map[string]string) }
    annotations["accord.io/sync-content-hash"] = incomingHash
    obj.SetAnnotations(annotations)

    // 4. Server-Side Apply
    // Apply the object to the Kubernetes cluster using Patch (Server-Side Apply preferred)
    return k8sClient.Patch(ctx, obj, client.Apply, client.FieldOwner("accord-sync-operator"))
}