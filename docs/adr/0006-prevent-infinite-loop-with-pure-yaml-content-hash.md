# ADR-0006: prevent infinite loop with pure-yaml content hash

## 1. 배경
Accord는 Kubernetes와 Git 간의 양방향 동기화를 수행합니다. Sync Operator가 Git의 변경 사항을 K8s 클러스터에 반영하면, K8s API 서버는 리소스 변경 이벤트를 발생시킵니다. 이때 Inventory Controller가 이를 감지하여 Git에 반영하려고 시도하면 infinite loop가 발생합니다.
이를 차단하고 시스템의 멱등성을 보장하기 위한 명백한 로직이 필요했습니다.

## 2. 결정
멱등성을 보장하는 key로 Git commit hash를 사용하지 않고, annotation 및 불필요한 metadata를 제거한 순수 YAML content를 사용하여 SHA-256 해시를 계산하고, 이를 Git commit hash 대신 사용하기로 결정했습니다.
- Sync Operator는 리소스를 K8s에 배포할 때, 배포할 순수 컨텐츠의 해시값을 계산하여 'accord.io/sync-content-hash'를 annotation으로 추가합니다.
- Inventory Controller는 K8s 이벤트를 수신하면, 대상 리소스의 metadata를 정규화하여 hash값을 계산하고, 이 값이 annotation의 hash값과 동일하다면 Reconsile 루프를 즉시 종료합니다.

## 3. 이유
- **Git 커밋 해시의 한계:** 하나의 Git 커밋에는 여러 개의 YAML 파일이 포함될 수 있으므로, 단일 리소스의 고유한 상태를 1:1로 대변하기 어렵다.
- **K8s 메타데이터의 변동성 배제:** K8s는 리소스 생성 및 수정 시 내부적으로 status, uid, resourceVersion, generation, managedFields 등의 동적 메타데이터를 끊임없이 주입하거나 변경한다. 이를 포함하여 상태를 비교하면, 실제 사용자가 의도한 설계(Spec)가 변하지 않았음에도 '변경됨'으로 오인하여 무한 루프가 발생한다.
- **정확한 상태 비교 (Content-Driven):** 시스템과 K8s가 자동으로 생성하는 껍데기를 모두 벗겨내고 순수 알맹이(Spec, 필수 Label 등)만으로 해시를 생성하면, Git과 K8s 간의 진정한 상태 일치 여부를 완벽하게 보장할 수 있다.

## 4. 결과
- Git -> K8s 배포 후 발생하는 K8s -> Git 역동기화(Echo)가 성공적으로 차단된다.
- Inventory Controller 내부에 수신된 K8s Object에서 K8s 종속적인 메타데이터를 필터링(Strip)하고 정규화하는 전처리 로직이 필수적으로 요구된다.
- 사용자가 accord.io/sync-content-hash 값을 임의로 수정하더라도, 컨트롤러가 실제 콘텐츠 기반으로 다시 덮어쓰므로 시스템의 무결성이 유지된다.

## 5. 구현
- 해시 알고리즘: `crypto/sha256`
- 어노테이션 키: `accord.io/sync-content-hash`

### 5.1. 메타데이터 필터링 및 해시 계산
```go
// internal/inventory/normalize.go
func normalizeKubernetesObject(obj map[string]interface{}) {
	delete(obj, "status") // status 속성 완전 제거

	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return
	}
    // K8s 종속적 동적 메타데이터 제거
	for _, k := range stripMetadataKeys { // uid, resourceVersion, generation 등
		delete(meta, k)
	}
	// ... (불필요한 annotation 및 metadata 속성 정리) ...
}
```