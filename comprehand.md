# accord 아키텍처 이해 요약 (1회 확인용)

이 문서는 `docs/architecture.md`를 읽고 정리한 **개인용 요약**이다. 구현·운영의 단일 기준은 항상 `docs/architecture.md`와 코드를 따른다.

## 무엇을 만드는가

**accord**는 Argo CD 환경을 대상으로, (1) **클러스터와 Git 간 양방향 형상 동기화**와 (2) **미러 환경의 승인 기반 안전 업그레이드·관측**을 목표로 한다. Git을 SSOT로 유지하되, 클러스터 직접 변경도 추적·반영할 수 있게 설계한다.

## 다섯 컴포넌트 (합치지 않음)

| 컴포넌트 | 역할 요약 | 실행 형태(문서 기준) |
|-----------|-----------|----------------------|
| **inventory-controller** | Argo CD 리소스 Watch, YAML 정규화·해시, Git Export(배치 커밋) | Deployment, replica 1 + 리더 선출 |
| **sync-operator** | Git Push Webhook 수신, 경로/Kind 검증, 클러스터 Apply(SSA) | Deployment(무상태) |
| **release-watcher** | GitHub 릴리즈 폴링, 알림, `MirrorUpgradeRequest` 생성 | CronJob 등 주기 실행 |
| **mirror-upgrader** | CR 승인 감지, 업그레이드 Job 생성·모니터링, 헬스/스모크, 로그 수집 Job 트리거 | Deployment(컨트롤러) + Job |
| **log-collector** | 지정 시간·네임스페이스 Pod 로그 수집·요약·아카이브 | Job(동적 트리거) |

컴포넌트 간 직접 REST 호출 대신 **CR 상태 변화**로 느슨하게 연결한다.

## 핵심 설계 원칙 (반드시 지켜야 할 것)

1. **SHA-256 해시 + 정규화된 매니페스트 캐시**로 양방향 동기화 시 **무한 루프를 끊는다**. sync-operator가 Apply한 결과로 들어오는 Watch 이벤트는 해시가 같으면 **no-op**한다.
2. **정규화**: `status`, `metadata.uid` / `resourceVersion` / `creationTimestamp` / `managedFields` / `generation`, 특정 시스템 annotation 등은 제거·정렬된 YAML 기준으로 해시한다.
3. **이벤트 디바운스·배치 Git Push**: RateLimitingQueue, 디바운스(문서 예: 5초), Push는 배치 워커가 묶어서 처리해 Git 충돌을 줄인다.
4. **sync-operator**는 Apply 전 **server-side dry-run**으로 검증을 시도한다(환경에 따라 신뢰도 전제가 있음).
5. **RBAC 최소화**: 컴포넌트별 ServiceAccount 분리(문서 표 참고).

## 데이터 흐름 한 줄 요약

- **클러스터 → Git**: inventory-controller가 정규화·해시 후 Git에 반영.
- **Git → 클러스터**: Webhook → sync-operator가 `inventory/applications/` 등 경로만 선별 Apply.
- **업그레이드**: release-watcher가 `MirrorUpgradeRequest` 생성 → 승인 → mirror-upgrader가 Job → 완료 후 log-collector Job.

## Git 저장소·파일 규칙

- 브랜치: **`main`**은 클러스터와 1:1, sync-operator는 **main Push만 신뢰**. 선택적으로 **`accord-auto-export`** 등으로 export·PR 전략.
- 경로 예: `inventory/<kind-plural>/<cluster>/<namespace>/<name>.yaml` — Webhook에서 경로로 빠르게 필터링.

## CRD (`MirrorUpgradeRequest`)

- **그룹/버전**: `ops.accord.io/v1alpha1` (아키텍처 초안과 부트스트랩 문서 정합).
- **Spec**: 대상 클러스터·네임스페이스, 현재/목표 버전, `approvalMode`(Manual|Auto), 선택적 로그 수집·아카이브 경로.
- **Status**: `phase` 상태 머신(Pending → Approved → Running → Succeeded/Failed, RolledBack 등), 메시지, 시작/완료 시각, 동적 Job 참조.

## 구현 로드맵(문서)

Phase 1에서 **inventory-controller + sync-operator + 해시 루프 차단**을 먼저 완성하고, 이후 release-watcher, CRD/업그레이드 파이프라인, log-collector 순으로 확장하는 것이 권장된다.
