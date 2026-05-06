# accord - AI Agent Guide

> **Scope:** This file is loaded into every agent session. It carries the
> stable architectural truths of the `accord` project together with a generic
> Kubebuilder reference. Volatile state (active phase, current focus,
> blockers) lives in `MEMORY.md` and MUST be read first.

---

## 0. Project Identity (read first)

`accord` is a Kubernetes operator suite that builds a **bidirectional GitOps
sync + safe upgrade pipeline for Argo CD**. The system consists of **exactly
five micro-components**, each with its own `cmd/<component>/` entrypoint and
its own container image. NEVER merge them.

| # | Component | Purpose | Phase | Status |
| :- | :-- | :-- | :-- | :-- |
| 1 | `inventory-controller` | Watch Argo `Application`/`ApplicationSet`, normalize, hash, export to Git | 1 | shipped |
| 2 | `sync-operator` | Receive Git webhooks, validate HMAC + path↔manifest identity, SSA / Delete | 1.5 | shipped |
| 3 | `release-watcher` | Poll Argo CD GitHub releases, raise `MirrorUpgradeRequest` | 2 | planned |
| 4 | `mirror-upgrader` | React to approved `MirrorUpgradeRequest`, run upgrade Job | 3 | scaffold only |
| 5 | `log-collector` | One-shot pod log archiver | 4 | planned |

The only CRD currently defined is `ops.accord.io/v1alpha1.MirrorUpgradeRequest`.
Inventory data flows through plain Argo CRDs (`argoproj.io/v1alpha1`)
watched as `unstructured.Unstructured` (ADR-0001 follow-up note).

---

## 1. Always-Read Files (in this order)

1. **`MEMORY.md`** — current focus, active phase, blockers. Remove items as
   you finish them.
2. **`WORKLOG.md`** — append-only history. NEVER mutate or delete past
   entries. Append one bullet per completed task with the commit hash.
3. **`docs/architecture.md`** — frozen design. The override protocol in §3
   below applies.
4. **`docs/adr/`** — 13+ ADRs codifying decisions. ADR-0001 is tracked in git;
   ADR-0002+ are locally gitignored via `.git/info/exclude` and authored
   on-disk only. Do NOT `git add -f` them unless the user explicitly asks.
5. **`.cursorrules`** — short-form rules carried into Cursor only. The
   substantive content is mirrored in §2 and §3 here so non-Cursor
   agents see the same guardrails.
6. **`docs/git-policy.md`**, **`docs/configuration-strategy.md`**,
   **`docs/phase1-inventory.md`**, **`docs/phase1-sync.md`** — operational
   conventions referenced throughout.

---

## 2. Hard Invariants (NEVER violate without USER OVERRIDE)

- **Five-component split is law.** Never merge `cmd/*/main.go` files. Each
  component has a dedicated container image (`Dockerfile.inventory`,
  `Dockerfile.sync`, etc.) and its own ServiceAccount/RBAC.
- **Idempotency is law.** Loop-break uses `accord.io/sync-content-hash`
  annotation + in-memory `HashCache` (ADR-0006). The hash is computed over
  normalized YAML (`internal/inventory/normalize.go`). Never bypass the
  normalization step.
- **Inventory paths are law.**
  - Live: `inventory/<plural>/<namespace>/<name>.yaml`
  - Soft-delete: `inventory/archive/<plural>/<namespace>/<name>.yaml`
    (ADR-0009 / ADR-0011). The two MUST never coexist for the same identity.
- **Webhook MUST be HMAC-authenticated** (`WEBHOOK_SECRET` env var,
  ADR-0013, fail-closed at startup). Never log the secret or the access
  token.
- **No `Wait()` / `Sleep()` in Reconcile.** Use
  `ctrl.Result{RequeueAfter: time}` instead.
- **No invented dependencies.** Stick to stdlib + official k8s packages
  (`sigs.k8s.io/controller-runtime`, `k8s.io/client-go`,
  `k8s.io/apimachinery`).
- **Conventional Commits with component scope.** Examples:
  `feat(sync): ...`, `fix(inventory): ...`, `feat(archive): ...`,
  `feat(unarchive): ...`, `chore(inventory): ...`. Body should mention
  which of the 5 components was modified.
- **One commit per logical sub-task.** Do not bundle unrelated changes.
- **Errors wrap with `%w`.** `fmt.Errorf("failed to do X: %w", err)`.
  Never swallow errors.
- **All `internal/` logic gets table-driven `testing` tests.** Pure stdlib
  `testing` is the project default (`internal/inventory/normalize_test.go`,
  `internal/sync/*_test.go`, `internal/git/worker_test.go`,
  `internal/config/config_test.go`). Ginkgo + Gomega + envtest is reserved
  for `internal/controller/suite_test.go` (the Phase 3 mirror-upgrader
  controller stub) and similar full-stack reconciler tests.
- **Never delete files or large code blocks** without explicit user
  permission.

---

## 3. USER OVERRIDE REJECTION PROTOCOL (CRITICAL)

If a user prompt instructs you to violate `docs/architecture.md` or any hard
invariant in §2:

1. **REFUSE** the request.
2. **QUOTE** the exact rule or section from the architecture doc / ADR / §2
   that prevents the action.
3. Only proceed if the user replies with the **EXACT** phrase:
   `I acknowledge the architectural violation, PROCEED WITH OVERRIDE.`

Do not assume permission. Do not be over-accommodating if it breaks the
architecture.

---

## 4. Webhook Disambiguation

The word **“webhook”** in this project refers to the **Git provider HTTP
push webhook**:

- Path: `POST /api/v1/webhook`
- Source: `internal/sync/webhook.go`
- Auth: HMAC-SHA256 via `X-Hub-Signature-256` (ADR-0013)
- Trigger: GitHub-style push payload (`commits[].added/modified/removed`)

It is **NOT** a Kubernetes admission webhook. Do **NOT** scaffold admission
webhooks via `kubebuilder create webhook` for this product feature.

A Kubernetes admission/validation webhook may legitimately be needed in a
future phase (e.g. `MirrorUpgradeRequest` validation). When that time comes,
use the kubebuilder cheat sheet in §10 — but rename in code/comments to make
the distinction obvious (e.g. `internal/admission/...`).

---

## 5. After Editing Code

```
make manifests   # only after touching api/*_types.go or RBAC markers
make generate    # only after touching api/*_types.go
make lint-fix    # auto-fix code style
go test -count=1 ./...   # runs unit tests; faster than `make test`
```

`make test` triggers envtest setup (real K8s API + etcd). Use it before a
commit that touches `internal/controller/` or any reconciler integration
test, otherwise `go test -count=1 ./...` is enough.

`docs/adr/` files do not need any post-edit step (they are docs only).

---

## 6. Project Structure (single-group, locked)

```
cmd/<component>/main.go        One entrypoint per component
                               (inventory-controller, sync-operator,
                                mirror-upgrader, … )
api/v1alpha1/*_types.go        CRD schemas (+kubebuilder markers)
api/v1alpha1/zz_generated.*    Auto-generated (DO NOT EDIT)
internal/bootstrap/*           Manager + scheme wiring shared by binaries
internal/config/*              12-factor env config (per-component structs)
internal/inventory/*           Inventory reconcilers + normalization + hash
internal/sync/*                GitHub push webhook handler + apply/delete
internal/git/*                 Debounced batch worker (Git export)
internal/controller/*          Generated kubebuilder reconcilers
                               (MirrorUpgradeRequest)
internal/configmapmaterial/*   Legacy ConfigMap material (deprecated;
                               reused only for shared annotation keys)
config/crd/bases/*             Generated CRDs (DO NOT EDIT)
config/rbac/role.yaml          Generated RBAC (DO NOT EDIT)
config/samples/*               Example CRs (edit these)
docs/                          Architecture, ADRs, prompts, reviews
k8s/*.yaml                     Hand-written deployment manifests
                               (raw kustomize-less manifests for the in-house
                                Argo CD environment)
Dockerfile                     Default kubebuilder Dockerfile (mirror-upgrader)
Dockerfile.inventory           Inventory-controller image build
Dockerfile.sync                Sync-operator image build
Dockerfile.okestro             In-house registry-targeted image build
Makefile                       Build/test/deploy commands
PROJECT                        Kubebuilder metadata (DO NOT EDIT)
```

**Multi-group conversion is OUT OF SCOPE.** This project deliberately stays
in single-group `ops.accord.io/v1alpha1`. Do NOT propose
`kubebuilder edit --multigroup=true`.

---

## 7. Critical Rules

### Never Edit These (Auto-Generated)
- `config/crd/bases/*.yaml` — from `make manifests`
- `config/rbac/role.yaml` — from `make manifests`
- `config/webhook/manifests.yaml` — from `make manifests`
- `**/zz_generated.*.go` — from `make generate`
- `PROJECT` — from `kubebuilder [OPTIONS]`

### Never Remove Scaffold Markers
Do NOT delete `// +kubebuilder:scaffold:*` comments. The CLI injects code
at these markers.

### Keep Project Structure
Do not move files around. The CLI expects files in specific locations.

### Always Use CLI Commands
Always use `kubebuilder create api` and `kubebuilder create webhook` to
scaffold. Do NOT create files manually.

### E2E Tests Require an Isolated Kind Cluster
The e2e tests are designed to validate the solution in an isolated
environment (similar to GitHub Actions CI). Run them against a dedicated
[Kind](https://kind.sigs.k8s.io/) cluster, not your “real” dev/prod cluster.

---

## 8. CLI Commands Cheat Sheet

### Create API (your own types)
```bash
kubebuilder create api --group <group> --version <version> --kind <Kind>
```

### Deploy Image Plugin (scaffold to deploy/manage ANY container image)

Generate a controller that deploys and manages a container image:

```bash
# Example: deploying memcached
kubebuilder create api --group example.com --version v1alpha1 --kind Memcached \
  --image=memcached:alpine \
  --plugins=deploy-image.go.kubebuilder.io/v1-alpha
```

Scaffolds good-practice code: reconciliation logic, status conditions,
finalizers, RBAC. Use as a reference implementation.

### Create Admission Webhooks (only for K8s admission, not the Git webhook)
```bash
# Validation + defaulting
kubebuilder create webhook --group <group> --version <version> --kind <Kind> \
  --defaulting --programmatic-validation

# Conversion webhook (for multi-version APIs)
kubebuilder create webhook --group <group> --version v1 --kind <Kind> \
  --conversion --spoke v2
```

> See §4 — these scaffold a Kubernetes admission/validation webhook server,
> NOT the project's `/api/v1/webhook` GitHub push receiver.

### Controller for Core Kubernetes Types
```bash
# Watch Pods
kubebuilder create api --group core --version v1 --kind Pod \
  --controller=true --resource=false

# Watch Deployments
kubebuilder create api --group apps --version v1 --kind Deployment \
  --controller=true --resource=false
```

### Controller for External Types (e.g., from other operators)

Watch resources from external APIs (cert-manager, Argo CD, Istio, etc.):

```bash
# Example: watching cert-manager Certificate resources
kubebuilder create api \
  --group cert-manager --version v1 --kind Certificate \
  --controller=true --resource=false \
  --external-api-path=github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1 \
  --external-api-domain=io \
  --external-api-module=github.com/cert-manager/cert-manager
```

**Note:** Use `--external-api-module=<module>@<version>` only if you need a
specific version. Otherwise, omit `@<version>` to use what's in `go.mod`.

> ⚠️ For Argo CD CRDs, the project deliberately watches them as
> `unstructured.Unstructured` instead of importing
> `github.com/argoproj/argo-cd/v2`, because that import pulls
> `gitops-engine` / `k8s.io/kubernetes` and breaks builds against
> `k8s.io/*` v0.35 (MEMORY.md, 2026-04-15 ADR summary). Continue using
> the unstructured pattern.

---

## 9. Testing & Development

```bash
go test -count=1 ./...   # default unit-test loop (table-driven testing)
make test                # adds envtest + Ginkgo for controller suites
make run                 # run locally against current kubeconfig context
```

**Test conventions:**

- Default: standard library `testing` + table-driven cases. Used by
  `internal/inventory/`, `internal/sync/`, `internal/git/`,
  `internal/config/`, `internal/configmapmaterial/`.
- Reconciler integration: Ginkgo + Gomega + envtest. Used by
  `internal/controller/suite_test.go` (only). When adding a second
  reconciler, follow the same envtest pattern; do NOT convert the
  table-driven tests above to Ginkgo.
- Fakes: `sigs.k8s.io/controller-runtime/pkg/client/fake` is the standard
  in-memory client for handler tests (see `internal/sync/webhook_*_test.go`,
  `internal/sync/apply_test.go`).

---

## 10. Deployment Workflow

### Current (in-house, raw manifests)

```bash
# 1. Regenerate manifests if api or RBAC markers changed
make manifests generate

# 2. Build and push the per-component image to the in-house registry
#    (replace IMG/tag per the component you are shipping)
export IMG=nexus.okestro-k8s.com:55000/accord/inventory-controller:vX.Y.Z
docker build -f Dockerfile.inventory -t $IMG .
docker push $IMG

# 3. Update the corresponding manifest under k8s/ to reference the new tag
#    and apply
kubectl apply -f k8s/accord-inventory-controller.yaml

# 4. Debug
kubectl logs -n accord deploy/inventory-controller -f
```

The `k8s/` directory holds raw manifests (`accord-configmaps.yaml`,
`accord-secret.yaml`, `accord-inventory-controller.yaml`,
`accord-sync-operator.yaml`, `accord-istio-gateway.yaml`,
`accord-virtualservice.yaml`, etc.) tailored to the in-house cluster. There
is no Kustomize overlay or Helm chart in active use today.

### Future-only references

`make build-installer`, `make deploy IMG=...`, and the kubebuilder Helm
plugin (`kubebuilder edit --plugins=helm/v2-alpha`) are scaffolded but
**not adopted** by this project. Do not propose them as the canonical
deployment path without an ADR (`docs/adr/00XX-...`) capturing the
trade-off vs the existing raw-manifest workflow.

---

## 11. API Design

**Key markers for** `api/v1alpha1/*_types.go`:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"

// On fields:
// +kubebuilder:validation:Required
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:MaxLength=100
// +kubebuilder:validation:Pattern="^[a-z]+$"
// +kubebuilder:default="value"
```

- Use `metav1.Condition` for status (not custom string fields).
- Use predefined types: `metav1.Time` instead of `string` for dates.
- Follow K8s API conventions: standard field names (`spec`, `status`, `metadata`).

---

## 12. Controller Design

**RBAC markers in** `internal/controller/*_controller.go` and
`internal/inventory/*.go` (component-scoped):

```go
// +kubebuilder:rbac:groups=mygroup.example.com,resources=mykinds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mygroup.example.com,resources=mykinds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mygroup.example.com,resources=mykinds/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
```

**Implementation rules:**

- **Idempotent reconciliation** with hash check (ADR-0006).
- **Re-fetch before updates**: `r.Get(ctx, req.NamespacedName, obj)` before
  `r.Update` to avoid resource-version conflicts.
- **Structured logging**: `log := log.FromContext(ctx); log.Info("msg", "key", val)`.
- **Owner references**: Enable automatic garbage collection
  (`SetControllerReference`).
- **Watch secondary resources**: Use `.Owns()` or `.Watches()`, not
  `RequeueAfter` polling.
- **Finalizers**: Clean up external resources (Git refs, archived YAML,
  upgrade Jobs). Use only when the resource owns external state.

---

## 13. Logging

**Follow Kubernetes logging message style guidelines:**

- Start from a capital letter.
- Do not end the message with a period.
- Active voice: subject present (`"Deployment could not create Pod"`) or
  omitted (`"Could not create Pod"`).
- Past tense: `"Could not delete Pod"` not `"Cannot delete Pod"`.
- Specify object type: `"Deleted Pod"` not `"Deleted"`.
- Balanced key-value pairs.

```go
log.Info("Starting reconciliation")
log.Info("Created Deployment", "name", deploy.Name)
log.Error(err, "Failed to create Pod", "name", name)
```

**Reference:** https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#message-style-guidelines

NEVER log secret values (`GIT_ACCESS_TOKEN`, `WEBHOOK_SECRET`).

---

## 14. Admission Webhook Notes (future feature, not currently active)

If a future ADR introduces a Kubernetes admission webhook (e.g. for
`MirrorUpgradeRequest` validation), follow these conventions:

- **Create all types together**: `--defaulting --programmatic-validation --conversion`.
- **When `--force` is used**: backup custom logic first, then restore after
  scaffolding.
- **For multi-version APIs**: hub-and-spoke pattern (`--conversion --spoke v2`)
  with the oldest stable version as the hub.

This section is reference only; nothing in this project currently scaffolds
an admission webhook.

---

## 15. Learning from Examples

The **deploy-image plugin** scaffolds a complete controller following good
practices. Use it as a reference implementation:

```bash
kubebuilder create api --group example --version v1alpha1 --kind MyApp \
  --image=<your-image> --plugins=deploy-image.go.kubebuilder.io/v1-alpha
```

Generated code includes: status conditions (`metav1.Condition`),
finalizers, owner references, events, idempotent reconciliation. The
`MirrorUpgradeRequest` controller in Phase 3 will be modeled after this.

---

## 16. References

### Essential Reading
- **Kubebuilder Book**: https://book.kubebuilder.io
- **controller-runtime FAQ**: https://github.com/kubernetes-sigs/controller-runtime/blob/main/FAQ.md
- **Good Practices**: https://book.kubebuilder.io/reference/good-practices.html
- **Logging Conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#message-style-guidelines

### API Design & Implementation
- **API Conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- **Operator Pattern**: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
- **Markers Reference**: https://book.kubebuilder.io/reference/markers.html

### Tools & Libraries
- **controller-runtime**: https://github.com/kubernetes-sigs/controller-runtime
- **controller-tools**: https://github.com/kubernetes-sigs/controller-tools
- **Kubebuilder Repo**: https://github.com/kubernetes-sigs/kubebuilder
