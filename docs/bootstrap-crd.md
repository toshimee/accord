# Agent Task: Phase 0 - Project Bootstrap & CRD Generation

## 1. Context & Role
You are an expert Go developer and Kubernetes Operator architect. Your task is to initialize a new Kubernetes Operator project using `kubebuilder` and define the custom resource (CRD) based on the exact specifications provided below. 

Do NOT hallucinate fields or logic. Follow standard Kubebuilder conventions strictly.

## 2. Step 1: Project Initialization
Run the following commands in the root directory to initialize the Go module and Kubebuilder project.
(If the project is already initialized, you may skip this step, but ensure the domain and repo match).

```bash
go mod init accord
kubebuilder init --domain accord.io --repo accord
```

## 3. Step 2: Create API & CRD
Scaffold the `MirrorUpgradeRequest` API. We need both the resource and the controller skeleton.

```bash
kubebuilder create api --group ops --version v1alpha1 --kind MirrorUpgradeRequest --resource=true --controller=true
```

## 4. Step 3: Define Go Structs for CRD
Open the generated `api/v1alpha1/mirrorupgraderequest_types.go` file. Replace the default `Spec` and `Status` structs with the following exact definitions.

### 4.1 Spec Definition
Define `MirrorUpgradeRequestSpec` with the following fields and JSON tags:
- `TargetCluster` (string, required): The name of the target cluster.
- `TargetNamespace` (string, required): The namespace where Argo CD is installed.
- `CurrentVersion` (string, required): The current version of Argo CD.
- `DesiredVersion` (string, required): The version to upgrade to.
- `ApprovalMode` (string, required): Allowed values are "Manual" or "Auto".
- `AutoCollectLogs` (bool, optional): Whether to trigger log collection after the job.
- `LogArchivePath` (string, optional): The storage path for archived logs.

### 4.2 Status Definition
Define `MirrorUpgradeRequestStatus` with the following fields and JSON tags:
- `Phase` (string, optional): Current state of the request. (e.g., Pending, Approved, Running, Succeeded, Failed, RolledBack).
- `Message` (string, optional): Human-readable message indicating details about the current phase.
- `StartedAt` (*metav1.Time, optional): When the upgrade job actually started.
- `CompletedAt` (*metav1.Time, optional): When the upgrade job finished.
- `JobRef` (string, optional): The name of the Kubernetes Job that was dynamically spawned.

### 4.3 Go Code Example (Reference)
Ensure appropriate kubebuilder validation markers (e.g., `+kubebuilder:validation:Enum`) are added where necessary, especially for `ApprovalMode` and `Phase`.

```go
// +kubebuilder:validation:Enum=Manual;Auto
ApprovalMode string `json:"approvalMode"`

// +kubebuilder:validation:Enum=Pending;Approved;Running;Succeeded;Failed;RolledBack
Phase string `json:"phase,omitempty"`
```

## 5. Step 4: Generate Manifests
After writing the Go code, run the following commands to generate the DeepCopy methods and the actual CRD YAML files in the `config/crd` directory.

```bash
make generate
make manifests
```

## 6. Success Criteria
Reply with a summary of the generated files once you successfully run `make manifests`. Do NOT proceed to write controller Reconcile logic yet. This task is strictly for bootstrapping the CRD structure.