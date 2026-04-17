### 📄 2. `docs/git-policy.md` (18절 4번 항목: Git 운용 규칙)
에이전트가 Git Export 모듈을 짤 때 참조할 파일 경로 및 커밋 메시지 정책입니다.

```markdown
# 🐙 Accord Project Git Policy

## 1. Export File Path Rule
When `inventory-controller` exports a resource to the Git repository, it MUST adhere to the following directory structure:
`inventory/<plural-kind>/<namespace>/<name>.yaml`

*Example:* `inventory/applications/argocd/my-app.yaml`
`inventory/applicationsets/argocd/cluster-addons.yaml`

## 2. Automated Commit Message Format
When the Batch Worker commits exported YAMLs, the commit message must follow this format:
`chore(inventory): sync <count> resources [skip ci]`

- **[skip ci]**: This tag is mandatory to prevent CI/CD pipelines from triggering recursively on our own automated commits.
- **Body:** List the updated files in the commit body (up to 10 lines max).

## 3. Branch Strategy
- All syncs target the `main` branch directly.
- Ensure the Git client performs a `Pull (Rebase)` before any `Push` to handle upstream changes gracefully and avoid merge commits.