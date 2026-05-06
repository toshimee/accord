# ADR-0005 How control preexist resources

## 1. 배경
Accord는 신규 클러스터뿐만 아니라 이미 운영 중인 클러스터에도 배포된다. 이 경우, Accord가 감시를 시작하기 전에 이미 Argo CD에 의해 생성된 리소스들이 존재한다. 이 리소스들은 Git 저장소에 백업되어 있지 않거나, Accord의 멱등성 해시(accord.io/sync-content-hash)가 없는 상태다. 이를 강제로 덮어쓰거나 무시할 경우 상태 불일치(Drift)가 발생하거나 운영 중인 서비스에 영향을 줄 수 있으므로, 기존 리소스를 안전하게 Accord 체계로 편입시킬 전략이 필요하다.

## 2. 결정
기존 리소스를 강제로 덮어쓰거나 무시하지 않고, 10초 디바운싱 큐와 멱등성 해시(Idempotency Hash) 원리를 활용하여 자연스럽게 Accord의 관리 대상으로 편입시킨다.
- **Git 연산:** 컨테이너 내 OS 의존성(`git` 바이너리)을 없애기 위해, 순수 Go 언어 기반 라이브러리인 `go-git`을 사용하여 백업(Export) 워크플로우를 수행한다.
- **Queue 관리:** client-go의 workqueue를 사용하여 10초간의 변경 사항을 병합(Debouncing) 처리한다.
- **상태 감지:** controller-runtime의 Manager와 Informer를 활용해 기동 시점에 클러스터의 모든 리소스를 List 연산으로 읽어들인다.
- **해시 검증:** crypto/sha256으로 계산된 해시가 없는 리소스는 '미등록 자산'으로 분류하여 즉시 Export 큐에 인입시킨다.

## 3. 이유
- **운영 연속성 보장:** 기존 리소스를 삭제하거나 변경하지 않고 그대로 Git에 반영하므로, 서비스 중단 없이 GitOps 체계로 전환할 수 있다.
- **신뢰할 수 있는 소스(SSOT) 확보:** 클러스터의 현재 상태를 기반으로 Git 저장소를 먼저 동기화함으로써, Git과 클러스터 간의 괴리를 0으로 만든 상태에서 운영을 시작할 수 있다.
- **Informer 패턴 활용:** Inventory Controller는 기동 시점에 List 연산을 통해 전체 리소스를 파악한다. 이때 해시값이 없는 리소스는 '신규 백업 대상'으로 간주되어 인메모리 큐에 쌓이므로, 별도의 마이그레이션 도구 없이도 자연스럽게 흡수된다.

## 4. 결과
- **초기 Git 커밋 대량 발생:** Accord 최초 배포 시, 기존에 존재하던 모든 Argo CD 리소스가 Git에 커밋되면서 일시적으로 Git Activity가 집중될 수 있다.
- **자연스러운 멱등성 확보:** 첫 번째 백업(Export)이 완료되면 Git에는 해당 리소스의 해시가 포함된 YAML이 저장되고, 이후 Sync Operator가 이를 수신하여 클러스터에 반영할 때 해시 어노테이션이 주입되면서 완전한 양방향 동기화 루프가 완성된다.

## 5. 구현
- client-go/util/workqueue: [Queue 구현 및 관리](https://github.com/kubernetes/client-go/blob/master/util/workqueue/)
- sigs.k8s.io/controller-runtime/pkg/reconcile: [Reconcile 패턴](https://github.com/kubernetes-sigs/controller-runtime/blob/HEAD/pkg/reconcile/reconcile.go)
- sigs.k8s.io/yaml: [YAML 파싱 및 직렬화](https://github.com/kubernetes-sigs/yaml)
- crypto/sha256: [SHA256 해싱](https://pkg.go.dev/crypto/sha256)

### 5.1. 해시 검증 및 기존 리소스 제어 (Reconciler)
```go
// internal/inventory/reconciler.go
if ann := obj.GetAnnotations(); ann != nil {
    // 이미 클러스터에 존재하던 리소스에 어노테이션 해시가 있다면,
    // 정규화된 현재 해시와 비교하여 불필요한 Git 덮어쓰기 무시
    if v, ok := ann[configmapmaterial.SyncContentHashAnnotationKey]; ok && v == currentHash {
        log.Info("Hash matched (annotation). Ignoring event to break loop")
        return ctrl.Result{}, nil
    }
}
```
