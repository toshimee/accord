# ADR-0008: Webhook Response Logging Strategy

## 1. 배경
Accord의 Sync Operator는 Git 저장소(GitLab, GitHub 등)로부터 발생하는 Webhook 이벤트를 수신하여 클러스터에 리소스를 배포(Apply)합니다. 다수의 리소스가 빈번하게 동기화되는 환경에서, 정상적으로 처리된 Webhook 요청마다 상세한 진행 상황("Webhook 수신됨", "Payload 파싱 완료", "Apply 성공" 등)을 컨테이너의 표준 출력(Stdout)으로 남길 경우 K8s 클러스터의 로그 수집 스택(Fluentd, Promtail 등)에 과도한 부하(Log Spam)를 유발합니다. 이는 스토리지 비용 증가를 초래할 뿐만 아니라, 실제 장애 발생 시 중요한 Error 로그를 식별하기 어렵게 만드는 원인이 됩니다.

## 2. 결정
Webhook의 정상 처리(Success) 시 Sync Operator의 Stdout 로깅을 최소화(또는 생략)하고, 처리 결과에 대한 상세 정보는 Webhook 요청자(Git 플랫폼)에게 반환하는 **HTTP 200 OK 상태 코드와 체계화된 JSON 응답(Response Body)**으로 대체하는 RESTful 설계를 채택했습니다.
- Stdout 로그는 시스템 오류(Error)나 비정상적인 데이터 유입(Warning) 시에만 기록합니다.
- HTTP 응답 JSON에는 처리된 파일 경로, 처리 상태(status), 상세 정보(detail)를 배열 형태로 포함하여 반환합니다.

## 3. 이유
- **로깅 부하 감소:** 정상적인 동기화 요청에 대한 상세 로그를 기록하지 않음으로써, K8s 클러스터의 로그 수집 스택에 부하를 줄이고 스토리지 비용을 절감합니다.
- **장애 식별 용이성:** 정상 로그가 줄어들어 실제 장애 발생 시 중요한 Error 로그를 쉽게 식별할 수 있습니다.
- **RESTful 설계:** Webhook 요청자에게 처리 결과를 반환하여, 비동기적인 동기화 작업에 대한 피드백을 제공합니다.

## 4. 결과
- Webhook 요청자(Git 플랫폼)는 HTTP 응답 JSON을 통해 처리 결과를 확인할 수 있습니다.
- Sync Operator의 Stdout 로그는 오류 발생 시에만 기록되어, 클러스터 운영자가 중요한 로그를 놓치지 않도록 합니다.

## 5. 구현
- HTTP 라우터 및 핸들러: Go 언어의 net/http 라이브러리를 활용한 Webhook 수신 엔드포인트 구현.
- JSON 응답 구조체 (Response Schema):
```go
type webhookResponse struct {
    Results []webhookResult `json:"results"`
}

type webhookResult struct {
    Path   string `json:"path,omitempty"`
    Status string `json:"status"`
    Detail string `json:"detail,omitempty"`
}
```
- 로깅 레벨 제어: go.uber.org/zap 또는 klog를 사용하여 정상 Webhook 수신 로직에서는 Info 레벨 호출을 최소화하고, 실패 시에만 Error 레벨로 스택 트레이스와 함께 로깅.
