# ADR-0004 Documentation sync

## 1. 배경
accord 프로젝트를 진행하던 중, inventory-controller와 sync-operator가 1차적으로 완성되고, live 구성 테스트까지 마친 상태에서 필요한 기능이 떠올랐습니다.
archive 기능을 추가해야 했습니다. 이는 기존 Memory.md, worklog.md, changelog.md를 포함하여 가장 중요한 architecture.md까지 수정이 필요한 상태였습니다.
이미 agent가 schema를 완성한 상태에서, 관련 문서의 영향을 최소화 하기 위해 어떤 것을 고려했는지에 대한 내용을 작성합니다.

## 2. 결정
기존 Markdown 문서들이 어떤 역할을 하고 있는지 명확하게 정의하고, archive 기능을 추가함에 있어서 각 문서에 어떤 내용을 추가해야 하는지 결정했습니다.
- architecture.md : 프로젝트에 대한 전체적인 설계를 담고 있으며, 해당 문서의 주도권은 사용자에게 있습니다. 이 문서는 시스템의 기초이므로 Agent가 수정하는 것이 아닌, 사용자가 수정을 검토하고 승인하는 과정을 거쳐야 합니다. 그래서 가장 먼저 이 문서에 archive 기능에 대한 내용을 기술했습니다.
- phase-*.md : 프로젝트 내의 기능에 대한 요구사항 명시와 스캐폴딩 역할을 하는 문서입니다. 이 문서는 archive 기능을 추가할 때 pseudo 코드 작성을 해둡니다. 그래서 이 문서에는 archive 기능에 대한 pseudo 코드를 작성했습니다.
- Memory.md : Agent의 단기기억을 담고 있는 문서입니다. 해야할 일을 체크하는 역할을 합니다. 이 문서는 prompt로 통제되게끔 구성했습니다.
- worklog.md : Agent의 장기기억을 담고 있는 문서입니다. 이 문서들은 진행중인 task를 기록하고, 프로젝트의 마일스톤 역할을 합니다. 이 문서는 prompt로 통제되게끔 구성했습니다.
- changelog.md : 프로젝트가 진행됨에 따라, 변화한 내용 (commit 같은 것)을 기록하는 문서입니다. 이 문서는 prompt로 통제되게끔 구성했습니다.

## 3. 실행
architecture.md에 archive 기능에 대한 내용을 기술하고, phase-1.md에 archive 기능에 대한 pseudo 코드를 작성했습니다. 그 다음 prompt 0006-phase1-archive.md를 사용하여 archive 기능을 구현했습니다.

## 5. 구현

### 5.1. 각 문서 반영 예시 (Pseudo)
```markdown
// architecture.md 예시
## Archive 기능
- 삭제된 리소스는 완전히 삭제하지 않고 `inventory/archive/` 경로로 이동시킨다.

// phase-1.md 예시 (Pseudo Code)
if apierrors.IsNotFound(err) {
    gitClient.Move(inventoryPath, archivePath)
}
```
