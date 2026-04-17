🚀 Phase 1 실행 프롬프트 (Agent Prompt)
(아래 텍스트를 복사하여 에이전트에게 전달하세요)

[Agent Instruction: Execute Phase 1 - Inventory Controller & Git Export]

We are now ready to execute Phase 1. Before writing any code, you MUST thoroughly read the following 3 specification documents:

docs/phase1-inventory.md

docs/git-policy.md

docs/configuration-strategy.md

Your Tasks:

Configuration: Implement internal/config/config.go to parse environment variables (like GIT_REPO_URL, BATCH_INTERVAL_SECONDS) following the configuration strategy.

Reconciler Refactoring: Refactor internal/controller/inventory_controller.go (or wherever the inventory reconcile logic currently resides). Remove the temporary ConfigMap MVP. Set it up to Watch Application and ApplicationSet from argoproj.io/v1alpha1.

Loop Break: Ensure the Reconciler correctly implements the Hash calculation and Loop Break logic (Idempotency) as specified in the Pseudo Code.

Git Batch Worker: Implement internal/git/worker.go with a debounce queue that exports normalized YAMLs to the correct inventory/... paths and uses the [skip ci] commit format.

Constraints & Memory:

Strictly follow .cursorrules.

Do NOT merge main.go files.

When finished, remove the completed items from the "Active Tasks" in MEMORY.md and append a detailed record of your commits to WORKLOG.md.

Acknowledge you have read the documents, and begin step-by-step implementation.