# ADR-0009: Soft Delete Archive Policy

## 1. 배경
클러스터 내에서 감시 중인 리소스(Application, ApplicationSet 등)가 삭제되었을 때, 이를 Git 저장소에서도 즉시 완전히 삭제(Hard-Delete)할 경우 시스템의 형상 추적성이 훼손됩니다. 운영자의 실수로 K8s 리소스가 삭제되었거나 장애가 발생했을 때, 파일 자체가 사라지면 Git 히스토리를 뒤져서 복원해야 하는 운영 상의 번거로움과 위험성이 존재합니다. 따라서 삭제된 리소스를 안전하게 보관하고 손쉽게 복구(Rollback)할 수 있는 정책이 필요했습니다.

## 2. 결정
- 클러스터 리소스 삭제 감지 시, Git에서 파일을 물리적으로 삭제하지 않고 별도의 아카이브 디렉토리로 이동시키는 Soft-Delete(아카이브) 정책을 채택합니다.
- 삭제 이벤트가 감지되면, 해당 리소스의 YAML 파일을 기존 inventory/ 경로에서 inventory/archive/ 하위의 동일한 상대 경로로 이동시킵니다.
- K8s API의 apierrors.IsNotFound 에러를 명시적으로 '삭제(Delete)' 이벤트로 해석하여 아카이브 워크플로우를 트리거합니다.

## 3. 이유
- **휴먼 에러 방어 및 복구 용이성**: 리소스가 아카이브 디렉토리로 이동할 뿐 데이터는 그대로 보존되므로, 추후 관리자가 해당 파일을 원래 경로로 다시 옮기기만 하면 Sync Operator의 자동 동기화(Auto-Sync)를 통해 클러스터에 즉각 복원됩니다.
- **GitOps 철학 준수**: 클러스터에서 일어난 '삭제'라는 상태 변화 역시 시스템의 중요한 라이프사이클 중 하나입니다. 이를 아카이브 디렉토리로의 '이동(Move)'이라는 명시적인 커밋으로 남김으로써 전체 인프라의 상태 변화를 투명하게 자산화할 수 있습니다.
- **Debounce 큐와의 매끄러운 통합**: 기존에 구축한 10초 디바운싱 인메모리 큐 로직을 해치지 않고, 큐의 처리 단계에서 삭제 대상 파일에 대한 액션만 git mv로 분기하여 처리할 수 있어 아키텍처적 일관성이 유지됩니다.

## 4. 결과
- Git 저장소 내에 inventory/archive/라는 별도의 네임스페이스 격리 공간이 생성되며, 삭제된 이력들이 안전하게 누적됩니다.
- Inventory Controller 내부에 존재 여부를 체크하여 아카이브 로직으로 분기하는 명시적인 예외 처리(Error Catching) 코드가 추가됩니다.
- 향후 아카이브된 파일이 수동으로 복원되어 다시 배포될 때(부활), 아카이브 디렉토리에 남아있는 잔여 파일을 정리하는 추가적인 정합성 보장 로직([ADR-0011] 리소스 부활 시 아카이브 정리)이 필요해집니다.

## 5. 구현
- **삭제 감지**: Reconcile 루프 내 `client.Get(...)` 호출 시 반환되는 `apierrors.IsNotFound(err)`를 명시적으로 Catch.
- **이동 명령**: Git Batch Worker에서 Hard-Delete 대신 `git mv` 명령어를 활용하여 경로 변경. (예: `inventory/applications/...` ➔ `inventory/archive/applications/...`)
- **커밋 컨벤션**: 아카이브 발생 시 `feat(archive): move deleted resource <name> to archive [skip ci]` 포맷의 커밋 메시지를 강제하여 CI 파이프라인의 불필요한 트리거를 방지.

### 5.1. 구현 로직
```go
// inventory/controllers/resource_controller.go

// 1. 삭제 감지 (isDeleted = true)
if apierrors.IsNotFound(err) {
    // IsDeleted가 true이므로 삭제 이벤트를 감지하고 아카이브 워크플로우 실행
    isDeleted = true
}

// 2. 아카이브 경로 생성
archivePath := strings.Replace(
    inventoryPath, 
    "inventory/", 
    "inventory/archive/", 
    1,
)

// 3. 리소스 삭제 및 아카이브
if err := k8sClient.Delete(ctx, resource); err != nil {
    if !apierrors.IsNotFound(err) {
        return fmt.Errorf("failed to delete resource %s: %w", name, err)
    }
    isDeleted = true // 실제로 삭제되었음을 확인
}

// 4. Git Batch Worker를 통한 아카이브
if isDeleted && !isArchived {
    if err := gitClient.Move(inventoryPath, archivePath); err != nil {
        return fmt.Errorf("failed to move archived resource %s to %s: %w", name, archivePath, err)
    }
}
```
