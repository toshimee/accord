# ADR-0013: Webhook HMAC 서명 검증 (Fail-Closed)

## 1. 배경
`sync-operator`는 외부 Git 플랫폼(GitHub 등)의 push webhook 이벤트를 수신하여 클러스터에 Server-Side Apply 또는 Delete를 수행한다. 이 엔드포인트(`POST /api/v1/webhook`)는 외부에 노출되며, 핸들러가 받는 페이로드는 곧바로 `argoproj.io/v1alpha1` 리소스의 생성/수정/삭제로 이어진다.

`docs/review/claude/0001-claude-phase1-review.md` §1.2에서 지적된 바와 같이, 현재 구현은 어떠한 출처 검증 없이 누구의 POST든 처리한다. Istio Gateway를 통해 외부에 노출된 상태에서 이 엔드포인트는 사실상 **인증 없는 SSA 트리거**이며, 다음과 같은 공격 벡터를 가진다.
- 임의의 `Application`/`ApplicationSet` 매니페스트 삽입(공격자 통제 Argo CD 형상으로 변조).
- `commits[].removed`를 위조한 페이로드로 운영 리소스 강제 삭제.
- 내부망에서 도달 가능한 누구라도 부주의로 SSA를 유발 가능.

## 2. 결정
1. **모든 webhook 페이로드는 HMAC-SHA256 서명을 포함해야 한다.** GitHub과 GitLab을 비롯한 주요 Git 제공자가 공유하는 `X-Hub-Signature-256: sha256=<hex>` 형식을 채택한다.
2. **검증은 페이로드 파싱 이전 단계**(raw body 위에서)에서 수행한다. 검증 실패 시 즉시 `401 Unauthorized` 반환, JSON 본문 없이 단일 에러 로그만 남긴다(ADR-0008).
3. **시크릿 누락 시 fail-closed.** `WEBHOOK_SECRET` 환경 변수가 비어 있거나 공백만 있으면 `sync-operator`는 기동 자체를 거부한다.
4. **Provider-agnostic naming.** 시크릿 환경 변수명은 `WEBHOOK_SECRET`을 단일 표준으로 한다(추후 GitLab `X-Gitlab-Token` 지원이 추가되더라도 같은 시크릿을 재사용한다).

## 3. 이유
- **Defense in depth.** 네트워크 정책(NetworkPolicy/Istio AuthorizationPolicy)만으로는 내부망에서 잘못 도달하는 트래픽을 막기 어렵다. 페이로드 출처 인증은 응용 계층에서도 필요하다.
- **Constant-time 비교.** `crypto/hmac.Equal`은 타이밍 사이드채널을 방지한다. 직접 `==` 비교는 사용하지 않는다.
- **표준 호환성.** GitHub Push 페이로드 외에도 GitHub Apps, Gitea, Bitbucket Cloud 등이 동일 헤더 규약을 사용한다. provider별 시크릿 분기를 막아 운영을 단순화한다.
- **Fail-closed 정책.** “시크릿이 없으면 검증을 생략한다”와 같은 silent-fallback은 오설정 시 위험을 누적시키므로 채택하지 않는다.

## 4. 결과
- `sync-operator` 시작 시 `WEBHOOK_SECRET`이 비어 있으면 `os.Exit(1)`. K8s에서는 CrashLoopBackOff로 노출되어 즉시 발견 가능.
- 정상 운영 흐름에서는 추가 RTT 없이(같은 요청 내에서 본문 위에 HMAC 계산만 추가) 통과.
- 잘못된 서명에 대해서는 응답 본문에 세부 사항을 노출하지 않는다(공격자에게 내부 상태 유출 방지). 운영자는 `kubectl logs`로 reason을 확인한다.
- Git 제공자 측에는 동일한 secret을 webhook 설정에 등록해야 한다(예: GitHub: Webhook → Secret 필드).

## 5. 구현
- 신규 함수 `internal/sync/hmac.go::VerifyHMACSignature(secret, headerValue, body)`.
- `WebhookHandler`는 `WebhookSecret string` 필드를 보유하고, `ServeHTTP`에서 본문 read 직후·`ParseGitHubPushPaths` 호출 직전에 검증을 수행한다.
- 새로운 설정 구조체 `internal/config.SyncOperatorConfig{ WebhookSecret, GitAccessToken }`. 기존 `InventoryControllerConfig`와 분리한다(`docs/review/claude/0001-claude-phase1-review.md` §2.1 권고).
- 표준 헤더 상수 `internal/sync.SignatureHeader = "X-Hub-Signature-256"`.

### 5.1 검증 로직 (요약)
```go
func VerifyHMACSignature(secret, headerValue string, body []byte) error {
    if secret == "" {
        return errors.New("hmac secret not configured")
    }
    if headerValue == "" {
        return fmt.Errorf("missing %s header", SignatureHeader)
    }
    if !strings.HasPrefix(headerValue, "sha256=") {
        return errors.New("invalid signature format")
    }
    received, err := hex.DecodeString(strings.TrimPrefix(headerValue, "sha256="))
    if err != nil {
        return fmt.Errorf("decode signature: %w", err)
    }
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    if !hmac.Equal(received, mac.Sum(nil)) {
        return errors.New("hmac mismatch")
    }
    return nil
}
```

### 5.2 핸들러 통합 순서
1. `r.Method == POST` 확인 → 아니면 405.
2. body read (32 MiB 제한).
3. **HMAC 검증** → 실패 시 401 + 단일 error 로그 + 즉시 return.
4. `ParseGitHubPushPaths`로 페이로드 파싱.
5. 기존 added/removed 처리 흐름 진입.

### 5.3 운영자 체크리스트
- [ ] 클러스터 시크릿 `accord-secret`에 `WEBHOOK_SECRET` 키 추가.
- [ ] GitHub 저장소 → Settings → Webhooks → Secret 동일 값 입력.
- [ ] `sync-operator` 재기동 후 webhook UI에서 **Recent Deliveries** 200 OK 확인.
- [ ] 의도적으로 secret을 잘못 설정한 테스트 push로 401이 기록되는지 확인.

## 6. 비고: 향후 확장
- GitLab(`X-Gitlab-Token`)은 별도 헤더이지만 동일 fail-closed 모델로 추가 가능. 헤더 검출 → secret 비교(평문 동일성)로 분기하면 된다.
- 외부에서 들어오는 HTTPS와 더불어, `sync-operator` 자체에도 네트워크 수신 측에 NetworkPolicy/AuthZ를 별도로 강제할 것을 권장한다(이중화).
