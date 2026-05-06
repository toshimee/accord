# ADR-0002: Container Build 시 ENTRYPOINT를 /manager로 통일

## 1. 배경 (Context)
프로젝트를 구축하며 여러 컴포넌트(Inventory Controller, Sync Operator 등)를 개별 컨테이너로 빌드해야 하는 상황입니다. 초기에는 컴포넌트별로 바이너리 명칭과 ENTRYPOINT를 제각각 다르게 작성할지 고민이 있었으나, 여러 불편함과 유지보수 오버헤드를 체감하여 표준화된 접근법이 필요해졌습니다.

## 2. 결정 (Decision)
모든 컨테이너 이미지의 빌드 결과물(바이너리)과 ENTRYPOINT를 `/manager`로 통일하여 사용하기로 결정했습니다.

## 3. 이유 (Reasoning)
- **Kubebuilder와 Kustomize 표준 규약**: 이 프로젝트는 Kubebuilder 프레임워크 기반으로 구축되었으며, 핵심 라이브러리인 `controller-runtime`의 주요 객체가 `manager`입니다. Kubebuilder 스캐폴딩의 기본값이 `/manager` 호출로 구성되어 있어, Kustomize 배포 매니페스트(Deployment)를 추가 수정 없이 재사용할 수 있습니다.
- **CI/CD 및 패키징 파이프라인 단순화**: 컴포넌트마다 바이너리 이름이 다르면, 빌드 및 배포 파이프라인(Makefile 등)에서 각각의 명칭을 명시해야 하므로 코드가 불필요하게 늘어납니다. 명칭을 통일함으로써 스크립트와 파이프라인을 단순하게 유지할 수 있습니다.
- **컨테이너 격리성**: x86 서버 환경과 달리, 컨테이너 환경에서는 프로세스 격리 덕분에 서로 다른 서비스라도 내부 바이너리 이름이 동일해도 충돌이나 제약이 전혀 없습니다.

## 4. 결과 (Consequences)
- Makefile 및 Dockerfile 구조가 단순해지고 전체적인 일관성을 확보했습니다.
- 향후 새로운 컴포넌트(Operator 등)가 추가되더라도 기존 빌드/배포 파이프라인 규약을 그대로 복사하여 손쉽게 확장할 수 있습니다.

## 5. 구현
```dockerfile
# Dockerfile 예시
# Kubebuilder 표준에 맞춰 ENTRYPOINT를 /manager로 고정
ENTRYPOINT ["/manager"]
```