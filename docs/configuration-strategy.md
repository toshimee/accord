# ⚙️ Accord Project Configuration Strategy

## 1. 개요 (Overview)
본 문서는 `accord` 시스템(Inventory, Sync, Upgrader 등)을 구성하는 컨트롤러들이 런타임 환경에서 설정값을 어떻게 주입받고 관리해야 하는지 정의한다. 모든 설정은 **12-Factor App 원칙**에 따라 코드와 엄격히 분리되며, 하드코딩(Hard-coding)은 절대 엄금한다.

## 2. 설정의 3단계 분류 (Configuration Tiers)

시스템의 설정은 그 성격과 보안 민감도에 따라 다음 3가지로 분류하여 쿠버네티스 리소스(`ConfigMap`, `Secret`, `Args`)로 매핑한다. 에이전트는 로직을 구현할 때 반드시 아래 명시된 방식(`os.Getenv` 또는 `flag`)으로 값을 읽어와야 한다.

### 2.1 시스템 일반 설정 (ConfigMap -> Environment Variables)
애플리케이션의 동작 방식을 결정하는 비민감 설정이다. 쿠버네티스의 `ConfigMap`을 통해 주입된다. 코드에서는 `os.Getenv`를 통해 가져오며, 값이 없을 경우의 **기본값(Default Fallback)** 처리가 필수적이다.

| Environment Variable | Description | Example / Default Value |
| :--- | :--- | :--- |
| `GIT_REPO_URL` | 형상을 동기화할 대상 Git 저장소의 URL | `https://github.com/example/argocd-ops.git` |
| `GIT_BRANCH` | 동기화 대상 브랜치 | `main` |
| `BATCH_INTERVAL_SECONDS` | Git Push 배치 워커의 실행 주기 (초) | `30` |
| `EXPORT_PATH_TEMPLATE` | Git Export 시 파일이 저장될 경로 템플릿 | `inventory/{{.Kind}}/{{.Namespace}}/{{.Name}}.yaml` |

### 2.2 민감한 자격 증명 (Secret -> Environment Variables)
Git 접근 권한이나 외부 API 통신에 필요한 민감 정보이다. 쿠버네티스의 `Secret` 리소스를 통해 파드의 환경 변수로 안전하게 주입된다. 코드상에서는 일반 환경 변수처럼 `os.Getenv`로 읽어오지만, **절대 로그(Log)나 에러 메시지에 해당 값을 노출(Print)해서는 안 된다.**

| Environment Variable | Description | Example |
| :--- | :--- | :--- |
| `GIT_USERNAME` | Git 인증에 사용할 봇(Bot) 또는 유저 이름 | `accord-bot` |
| `GIT_ACCESS_TOKEN` | Git 인증용 Personal Access Token (PAT) | `ghp_xxxxxxxxxxxx` |

### 2.3 실행 인자 (CLI Flags)
Kubebuilder 및 `controller-runtime` 프레임워크가 기본적으로 제공하거나, 애플리케이션 시작 시점에 컨트롤러 매니저(Manager)를 튜닝하기 위한 설정이다. `cmd/<component>/main.go`에서 `flag` 패키지를 통해 바인딩된다.

| Flag | Description | Default Value |
| :--- | :--- | :--- |
| `--metrics-bind-address` | 프로메테우스 메트릭을 노출할 포트 | `:8080` |
| `--health-probe-bind-address` | Liveness/Readiness 프로브 포트 | `:8081` |
| `--leader-elect` | 다중 레플리카 환경에서의 리더 선출 활성화 여부 | `false` (운영 환경에선 `true`) |

## 3. 에이전트 구현 지침 (Agent Implementation Guide)

1. **초기화 시점 로드:** 환경 변수는 `main.go`의 `init()` 또는 `main()` 함수 시작 지점에서 읽어와야 한다.
2. **Config 구조체 활용:** 개별 변수를 여기저기서 `os.Getenv`로 호출하지 말고, `internal/config/config.go`를 생성하여 환경 변수를 파싱하고 검증(Validation)하는 전용 구조체(Struct)를 만들어라.
3. **의존성 주입 (DI):** 파싱된 Config 객체를 `InventoryReconciler`나 `GitBatchWorker` 생성 시점에 주입(Inject)하여 의존성을 명확히 하라.

```go
// Example of Config Struct
type ControllerConfig struct {
    GitRepoURL    string
    GitBranch     string
    BatchInterval time.Duration
    GitToken      string // NEVER log this
}
```