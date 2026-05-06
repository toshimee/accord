# ADR-0012: Git 주도 삭제 동기화

## 1. 배경
Accord의 양방향 동기화 환경에서, 사용자가 K8s 클러스터의 리소스를 직접 삭제하면 Inventory Controller가 이를 감지하여 Git에 아카이브 처리합니다([ADR-0009]). 반면, 사용자가 클러스터 접근 없이 Git 저장소에서 직접 YAML 파일을 삭제(또는 관리 경로 외부로 이동)하고 커밋을 푸시하는 시나리오가 존재합니다.
이때 Sync Operator가 웹훅을 수신하더라도 변경/추가된 파일이 없다는 이유로 무시한다면, 클러스터에는 해당 리소스가 그대로 활성 상태로 남게 됩니다. 이는 Git의 통제를 벗어난 '고아(Orphan) 리소스'를 양산하며, 심각한 상태 불일치(Drift)와 보안/운영 상의 리스크를 초래합니다.

## 2. 결정
1. **Sync Operator 확장:** 웹훅 페이로드에서 `commits[].removed` (삭제된 파일) 목록을 추출하여, 대상 클러스터 내의 해당 리소스 역시 동기화하여 명시적으로 삭제(`client.Delete`) 처리합니다.
2. **거버넌스 안전장치 (Shift-Left):** 클러스터에서의 복구 불가능한 일괄 삭제를 방지하기 위해, K8s 컨트롤러에 방어 로직을 넣는 대신 Git 저장소(GitHub/GitLab) 레벨에서 브랜치 보호(Branch Protection) 및 필수 PR 리뷰(CODEOWNERS) 정책을 강제하여 휴먼 에러를 차단합니다.

## 3. 이유
- **단일 진실 공급원(SSOT) 원칙 준수**: Git 저장소에서 매니페스트가 제거되었다는 것은 인프라의 폐기를 의미하므로, 클러스터의 상태도 이에 즉각적으로 동기화되어야 합니다.
- **경로 기반 식별 아키텍처의 효용성**: 파일 내용을 읽을 수 없는 상황이더라도, 디렉토리 구조(`inventory/<kind-plural>/<namespace>/<name>.yaml`) 덕분에 경로 문자열 파싱만으로 K8s 리소스를 정확히 역추산할 수 있습니다.

## 4. 결과
- Git 저장소에서의 파일 삭제만으로 K8s 클러스터의 리소스를 안전하고 깔끔하게 제거할 수 있는 완전한 GitOps 파이프라인이 완성됩니다.
- Sync Operator가 리소스를 삭제하면 Inventory Controller가 이를 감지하여 아카이브 로직을 시도합니다. 하지만 Git 워커가 디스크에서 `archive/`로 이동시킬 때 대상 파일이 이미 Git에서 삭제되어 없음을 인지하고 무시하므로 무한 루프가 발생하지 않습니다.

## 5. 구현 로직 (Pseudo Code)
```go
// cmd/sync-operator/ 내부 웹훅 핸들러

// 1. 삭제된 파일 목록 처리
for _, removedPath := range commit.Removed {
    // inventory/ 하위 경로인지 확인 (archive 제외)
    if !strings.HasPrefix(removedPath, "inventory/") || strings.HasPrefix(removedPath, "inventory/archive/") {
        continue
    }

    // 2. 경로 기반 식별자 재구성 (3-depth 구조)
    // parts: ["applications", "namespace", "resource-name.yaml"]
    parts := strings.Split(strings.TrimPrefix(removedPath, "inventory/"), "/")
    if len(parts) != 3 { continue }
    
    kindPlural := parts[0] // e.g., "applications" or "applicationsets"
    namespace := parts[1]  // e.g., "default"
    name := strings.TrimSuffix(parts[2], ".yaml") // e.g., "test-app"

    k8sKind := "Application"
    if kindPlural == "applicationsets" { k8sKind = "ApplicationSet" }

    // 3. K8s Delete API 호출 (빈 껍데기 객체 생성 후 삭제)
    obj := &unstructured.Unstructured{}
    obj.SetGroupVersionKind(schema.GroupVersionKind{
        Group:   "argoproj.io",
        Version: "v1alpha1",
        Kind:    k8sKind,
    })
    obj.SetNamespace(namespace)
    obj.SetName(name)

    if err := k8sClient.Delete(ctx, obj); err != nil {
        if !apierrors.IsNotFound(err) {
            log.Error(err, "failed to delete resource via Git deletion")
        }
    }
}