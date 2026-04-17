# 🧩 Phase 1: Inventory Controller Specification

## 1. Component Overview
- **Binary:** `cmd/inventory-controller/main.go`
- **Goal:** Watch Argo CD resources, compute hashes to break infinite loops, and export to Git.
- **Target Resources:** `Application` and `ApplicationSet` from `argoproj.io/v1alpha1`.

## 2. Reconcile Loop Pseudo Code (Strict Idempotency)
The Reconciler MUST follow this exact logic to prevent Bidirectional Sync infinite loops.

```go
func (r *InventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the Resource (Application or ApplicationSet)
    obj := fetchResource(req)
    if notFound { return ctrl.Result{}, nil }

    // 2. Normalize and Hash (Using internal/inventory/normalize.go)
    normalizedYAML := Normalize(obj)
    currentHash := ComputeSHA256(normalizedYAML)

    // 3. Loop Break Check (CRITICAL)
    // Compare with in-memory cache OR annotation `accord.io/sync-content-hash`
    if IsHashMatchedInCache(req.NamespacedName, currentHash) {
        log.Info("Hash matched. Ignoring event to break loop.", "hash", currentHash)
        return ctrl.Result{}, nil // Stop processing!
    }

    // 4. Update Cache
    UpdateCache(req.NamespacedName, currentHash)

    // 5. Send to Git Export Queue (Do NOT push to Git synchronously in this loop)
    r.GitExportQueue.Enqueue(ExportJob{
        Path:    fmt.Sprintf("inventory/%s/%s/%s.yaml", obj.Kind, obj.Namespace, obj.Name),
        Content: normalizedYAML,
    })

    return ctrl.Result{}, nil
}