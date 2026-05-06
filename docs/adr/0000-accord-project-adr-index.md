# Accord Project ADR (Architecture Decision Record) Index
이 문서는 Accord 프로젝트를 진행하며 결정된 주요 아키텍처, 설계 철학, 트러블슈팅 및 운영 정책을 기록한 ADR 문서들의 목차입니다.

## 🏗️ 1. Core Architecture & Workflow (핵심 설계 및 워크플로우)
* **[ADR-0001]** inventory-in-memory-queue-design.md: 외부 브로커 없이 Inventory Controller에 인메모리 큐 및 10초 디바운싱 설계를 채택한 이유
* **[ADR-0003]** prompt-logic-versus.md: Prompt(행동 지시)와 Logic/Spec(설계도)의 명확한 역할 분리
* **[ADR-0004]** documentation-sync.md: 코드와 설계의 괴리를 막는 3단계 문서 동기화 전략 (Architecture, Memory, Worklog)
* **[ADR-0005]** how-control-preexist-resources.md: Accord 도입 전 기존 배포된 리소스(Pre-existing)들의 자연스러운 제어 및 양방향 통제 흡수 전략

## ⚙️ 2. Synchronization & Idempotency (동기화 및 멱등성)
* **[ADR-0006]** prevent-infinite-loop-with-pure-yaml-content-hash.md: 무한 루프 방지를 위한 멱등성 키로 'Git 커밋 해시' 대신 '순수 YAML 콘텐츠 해시(SHA-256)'를 채택한 이유
* **[ADR-0007]** handling-multiple-resources-in-webhook.md: 단일 Git 커밋 내에 여러 YAML이 수정되었을 때, 뭉텅이 처리 대신 개별 파싱 및 배포(Apply)를 수행하는 설계
* **[ADR-0008]** webhook-response-logging-strategy.md: 웹훅 성공 시 Stdout 로깅을 최소화하고 HTTP 200 JSON 응답을 채택한 RESTful 설계 이유

## 🛡️ 3. Edge Cases & State Consistency (엣지 케이스 및 상태 정합성)
* **[ADR-0009]** soft-delete-archive-policy.md: 클러스터 리소스 삭제 시, Git에서 Hard-Delete 대신 `archive/` 디렉토리로 이동시키는 Soft-Delete 정책 채택 이유
* **[ADR-0010]** race-condition-squash-defense.md: 사용자가 Apply 직후 Delete를 연타할 때, K8s 상태 기반(Level-Trigger) 특성과 디바운싱을 활용한 플래핑(Flapping) 방어 원리
* **[ADR-0011]** resurrection-unarchive-handling.md: 아카이브된 리소스가 클러스터에 재배포(부활)될 경우, 기존 아카이브 파일을 정리하여 상태 불일치를 막는 로직
* **[ADR-0012]** git-driven-delete-sync.md: Git 저장소에서 수동으로 파일이 삭제되었을 때, 클러스터 리소스의 고아화(Orphan)를 막기 위한 동기화 정책

## 🌐 4. Network & Security (네트워크 라우팅 및 보안)
* **[ADR-0013]** sync-operator-tls-secret-mount.md: K8s 내부 Webhook 서버 구동을 위해 자체 서명(Self-signed) TLS 인증서를 생성하고 Secret으로 마운트한 이유
* **[ADR-0014]** istio-tls-codec-error-resolution.md: Istio IngressGateway와 백엔드 간 TLS 코덱 충돌(503 에러) 해결을 위해 Service 포트 네이밍(`https-`)을 강제한 이유
* **[ADR-0015]** virtualservice-routing-without-rewrite.md: 웹훅 라우팅 시 404 에러 방지를 위해 URI Rewrite를 배제하고 Exact Match 라우팅을 적용한 이유

## 🚀 5. Build, Test & Deployment (빌드 및 테스트 전략)
* **[ADR-0002]** why-use-entrypoint-manager.md: 컨테이너 빌드 시 ENTRYPOINT를 `manager`로 통일한 이유 (Kubebuilder 규약 및 CI/CD 단순화)
* **[ADR-0016]** multi-component-dockerfile-separation.md: 단일 Dockerfile에서 ARG를 주입하는 방식 대신, 컴포넌트별로 Dockerfile(`inventory`, `sync`)을 물리적으로 분리한 이유
* **[ADR-0017]** clone-cluster-testing-strategy.md: 운영 환경과의 동기화 충돌(Sync War) 방지를 위해, Argo CD 봇을 비활성화한 Data-Only Clone 클러스터에서 테스트를 격리한 전략