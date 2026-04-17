내가 무슨 프로젝트를 하고 있고, 해야할 다음 작업이 어떻게 되는지 확인 바랍니다.

진행된 내용은 체크 표시도 바랍니다.

internal/inventory/normalize.go에 YAML 정규화 로직을 짤 건데, 코드보다 무조건 Unit Test 코드를 먼저 작성합니다 (TDD). 시스템 필드(status, uid 등)가 섞인 YAML과 알맹이만 있는 YAML을 넣었을 때 동일한 SHA-256 해시가 나오는지 검증하는 테스트를 통과해야만 합니다.

