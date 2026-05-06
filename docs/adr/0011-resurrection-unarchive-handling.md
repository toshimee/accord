# ADR-0011: Resurrection Unarchive Handling

## 1. 배경
[ADR-0009]의 결정에 따라, 클러스터에서 삭제된 리소스는 Git 저장소의 inventory/archive/ 경로에 보관됩니다. 만약 사용자가 삭제 실수를 인지하고 동일한 리소스를 클러스터에 다시 배포(Apply)하여 리소스가 '부활(Resurrection)'하게 된다면, Inventory Controller는 이를 새로운 Add 이벤트로 감지합니다.
이때 컨트롤러가 단순히 활성 경로(inventory/)에 새로운 YAML 파일을 생성하기만 한다면, inventory/와 inventory/archive/ 양쪽 모두에 동일한 리소스의 파일이 존재하게 됩니다. 이는 단일 진실 공급원(SSOT) 원칙을 위배하며, 이후 동기화 과정에서 치명적인 상태 불일치(State Inconsistency)를 유발할 수 있습니다.

## 2. 결정
Inventory Controller가 클러스터의 리소스 생성 및 수정(Add/Update) 이벤트를 처리할 때, 해당 리소스가 아카이브 디렉토리에 존재하는지 확인하고 복원(Unarchive)하는 선행 로직을 추가합니다.
정규화된 YAML을 inventory/ 경로에 저장하기 직전에, inventory/archive/ 하위에 동일한 파일이 있는지 검사합니다.
아카이브된 파일이 발견되면, 단순히 새 파일을 생성하는 대신 해당 파일을 원래의 활성 경로로 다시 이동(git mv)시킨 후 최신 상태로 덮어씁니다.
## 3. 이유
- 상태 정합성 보장 (SSOT 유지): 동일한 리소스에 대한 매니페스트가 활성 디렉토리와 아카이브 디렉토리에 중복으로 존재하는 것을 원천적으로 차단합니다.
- Git 라이프사이클 히스토리 보존: 파일을 삭제하고 새로 만드는 대신 git mv를 사용하여 되돌림으로써, 해당 리소스가 '생성 ➔ 아카이브 ➔ 복원 ➔ 수정'된 전체 라이프사이클이 단일 파일의 Git 히스토리(Log)로 끊김 없이 이어집니다.
- 충돌 방지: Sync Operator가 전체 디렉토리를 스캔할 때 발생할 수 있는 중복 적용(Duplicate Apply) 및 참조 에러를 방지합니다.
## 4. 결과
- 클러스터에 리소스가 다시 배포되는 즉시, Git 저장소에서도 아카이브에 있던 파일이 제자리로 돌아오면서 완벽한 상태 일치를 이룹니다.
- Inventory Controller의 Reconcile 및 Batch Worker 로직 내에 파일 존재 여부를 확인하는 디스크 I/O 작업(os.Stat 등)이 미세하게 추가됩니다.
- 삭제와 부활이 빈번하게 일어나는 리소스의 경우, 파일 이동에 따른 커밋이 추가로 발생하지만, 이는 아키텍처적으로 올바른 동작입니다.
## 5. 구현
- 존재 여부 검사: 파일 저장 전 os.Stat(archivePath)를 호출하여 아카이브 파일 존재 여부 확인.
- 이동 명령: 아카이브 파일이 존재할 경우 go-git 또는 git mv 명령어를 통해 inventory/archive/applications/... ➔ inventory/applications/... 경로로 복원.
- 파일 갱신: 복원 후, 최신 해시와 설정값이 반영된 정규화 YAML로 파일 내용(Content)을 덮어쓰기(Write).
- 커밋 컨벤션: 복원 발생 시 feat(unarchive): restore resurrected resource <name> [skip ci] 포맷의 커밋 메시지를 사용하여 명확한 이력 추적성 제공.

### 5.1. 구현 로직
```go
// inventory/controllers/resource_controller.go

// 1. 아카이브 파일 존재 여부 확인
isArchived, err := gitClient.FileExists("inventory/archive/" + inventoryPath)
if err != nil {
    return fmt.Errorf("failed to check archive file existence: %w", err)
}

// 2. 아카이브된 파일이 존재하고, 리소스가 실제로 존재할 경우 (부활 시)
if isArchived && !apierrors.IsNotFound(err) {
    // Inventory 경로로 복원
    if err := gitClient.Move("inventory/archive/" + inventoryPath, inventoryPath); err != nil {
        return fmt.Errorf("failed to move archived resource %s to %s: %w", name, inventoryPath, err)
    }
}

// 3. 최신 상태로 파일 내용 갱신
if err := file.Content.Write(inventoryPath); err != nil {
    return fmt.Errorf("failed to write normalized YAML to %s: %w", inventoryPath, err)
}

// 4. 복원(Unarchive) 이벤트 커밋
if isArchived {
    if err := gitClient.Commit(
        fmt.Sprintf("feat(unarchive): restore resurrected resource %s [skip ci]", name),
        " Accord System", 
        "",
    ); err != nil {
        return fmt.Errorf("failed to commit unarchive: %w", err)
    }
}
```