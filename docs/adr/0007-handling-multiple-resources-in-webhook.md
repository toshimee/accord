# ADR-0007: Handling Multiple Resources in Webhook

## 1. 배경
Git 저장소에서 발생하는 단일 커밋(Commit)이나 병합(Merge) 이벤트는 여러 개의 YAML 파일 변경을 포함할 수 있습니다. 웹훅(Webhook) 페이로드에는 수정된 파일 목록이 배열 형태로 전달되는데, 이를 하나의 덩어리로 묶어 처리하거나 일부만 처리할 경우 특정 리소스의 반영이 누락되거나 동기화 상태가 불일치하는 문제가 발생할 수 있습니다. 특히 각각의 리소스마다 고유한 해시 검증이 필요하므로, 다수 파일을 안전하게 처리할 수 있는 구조가 필요합니다.

## 2. 결정
단일 웹훅 요청이 들어왔을 때, 변경된 파일 목록을 순회하며 각 YAML 파일을 개별적으로 파싱하고 클러스터에 배포(Apply)하는 '반복 처리(Iterative Processing)' 방식을 채택합니다.
- 웹훅 페이로드의 commits 또는 added/modified 리스트를 추출하여 루프를 실행합니다.
- 각 파일별로 독립적인 Reconcile 로직을 적용하여, 개별 리소스의 accord.io/sync-content-hash를 계산하고 반영합니다.

## 3. 이유
- 리소스 독립성 확보: 각 리소스는 서로 다른 상태와 해시값을 가집니다. 개별적으로 처리함으로써 특정 파일에서 오류가 발생하더라도 다른 파일의 동기화에 영향을 주지 않고 독립적인 상태 관리가 가능합니다.
- 부분 성공/실패 추적 용이: 어떤 파일이 성공적으로 반영되었고 어떤 파일에서 문제가 생겼는지 명확하게 식별할 수 있습니다. 이는 이후 작성할 [ADR-0008]의 JSON 응답 결과와 연계되어 디버깅 효율을 높여줍니다.
- Kubernetes API 최적화: K8s API 서버는 기본적으로 개별 리소스 단위의 요청을 받습니다. 루프를 통한 개별 처리는 클러스터의 표준 처리 방식과 일치하며, 대규모 변경 시에도 안정적인 트랜잭션 관리가 가능합니다.

## 4. 결과
- 웹훅 핸들러 내에 파일 목록을 처리하기 위한 반복문 로직이 추가됩니다.
- 단일 커밋에 너무 많은 파일이 포함될 경우 처리 시간이 다소 길어질 수 있으나, 데이터의 정합성과 안정성 측면에서 훨씬 유리합니다.
- 각 파일 처리 결과(성공/실패)를 수집하여 최종적으로 통합된 웹훅 응답을 생성해야 하는 추가적인 구현 공수가 발생합니다.

## 5. 구현
- Payload Parsing: GitHub/GitLab용 Webhook Payload 구조체를 활용하여 변경된 파일 경로 목록 추출
- Iteration Logic: `range` 문을 통해 파일 순회 및 개별 `Apply`
- Error Handling: 에러 발생 시 `results` 배열에 `status: error` 와 함께 누적

### 5.1. Webhook 순회 처리 로직
```go
// internal/sync/webhook.go
var results []webhookResult
for _, p := range paths {
    // 1. 각 파일별 실제 콘텐츠 패치
    raw, err := FetchGitHubRawFile(ctx, httpClient, h.GitHubToken, fullName, sha, p)
    if err != nil {
        results = append(results, webhookResult{Path: p, Status: "error", Detail: err.Error()})
        continue
    }
    
    // 2. K8s 클러스터 개별 배포 (Apply)
    if err := ApplyInventoryYAML(ctx, h.K8s, raw); err != nil {
        results = append(results, webhookResult{Path: p, Status: "error", Detail: err.Error()})
        continue
    }
    
    // 3. 정상 반영 결과 누적
    results = append(results, webhookResult{Path: p, Status: "applied"})
}
```