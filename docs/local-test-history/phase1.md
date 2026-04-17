# 로컬 중간 테스트 방법

## 1. Git 준비
테스트용 빈 GitHub 레포지토리를 하나 파거나, 로컬 디렉토리를 사용합니다.
## 2. 실행
로컬 터미널에서 환경 변수를 임시로 주고 컨트롤러를 실행합니다.
```
# 1. 기본 변수
export GIT_REPO_URL="https://github.com/toshimee/imsi.git"
export BATCH_INTERVAL_SECONDS="10"

# 2. 인증 변수 (configuration-strategy.md에 정의했던 값)
export GIT_USERNAME="toshimee" 
export GIT_ACCESS_TOKEN="ghp_여기에_발급받은_토큰을_넣어주세요"

# 3. 실행
go run ./cmd/inventory-controller/main.go
```
## 3. 트리거 (Cluster 조작)
로컬 쿠버네티스(Docker Desktop, minikube 등)에 더미 Application 리소스를 kubectl apply로 생성해 봅니다.
```더미 Application 리소스 생성
cat <<EOF | kubectl apply -f -
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
EOF
```
## 4, 결과 확인
컨트롤러 로그에 "큐에 담겼다"는 메시지가 뜨는지 확인.
10초 뒤 Git 저장소에 inventory/applications/네임스페이스/앱이름.yaml 파일이 [skip ci] 커밋 메시지와 함께 잘 올라갔는지 확인


# 에러
## 1. 원격 레포 깡통 
```
2026-04-17T15:35:08+09:00	ERROR	git-batch-worker	Git export batch failed; paths will be retried on next flush	{"error": "git clone: remote repository is empty"}
accord/internal/git.(*BatchWorker).flush
	/Users/gabri/accord/internal/git/worker.go:105
accord/internal/git.(*BatchWorker).Start
	/Users/gabri/accord/internal/git/worker.go:83
sigs.k8s.io/controller-runtime/pkg/manager.(*runnableGroup).reconcile.func1
	/Users/gabri/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.23.3/pkg/manager/runnable_group.go:260
```
## 2. 원격 레포 권한 미부여
```
2026-04-17T15:35:39+09:00	ERROR	git-batch-worker	Git export batch failed; paths will be retried on next flush	{"error": "git push: authentication required: No anonymous write access."}
accord/internal/git.(*BatchWorker).flush
	/Users/gabri/accord/internal/git/worker.go:105
accord/internal/git.(*BatchWorker).Start
	/Users/gabri/accord/internal/git/worker.go:83
sigs.k8s.io/controller-runtime/pkg/manager.(*runnableGroup).reconcile.func1
	/Users/gabri/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.23.3/pkg/manager/runnable_group.go:260
```

# 성공
## 1. 원격 푸시 성공
go run ./cmd/inventory-controller/main.go
2026-04-17T15:47:56+09:00	INFO	setup	Starting inventory-controller manager
2026-04-17T15:47:56+09:00	INFO	starting server	{"name": "health probe", "addr": "[::]:8081"}
2026-04-17T15:47:56+09:00	INFO	Starting EventSource	{"controller": "accord-inventory-application", "controllerGroup": "argoproj.io", "controllerKind": "Application", "source": "kind source: *unstructured.Unstructured"}
2026-04-17T15:47:56+09:00	INFO	Starting EventSource	{"controller": "accord-inventory-applicationset", "controllerGroup": "argoproj.io", "controllerKind": "ApplicationSet", "source": "kind source: *unstructured.Unstructured"}
2026-04-17T15:47:57+09:00	INFO	Starting Controller	{"controller": "accord-inventory-applicationset", "controllerGroup": "argoproj.io", "controllerKind": "ApplicationSet"}
2026-04-17T15:47:57+09:00	INFO	Starting workers	{"controller": "accord-inventory-applicationset", "controllerGroup": "argoproj.io", "controllerKind": "ApplicationSet", "worker count": 1}
2026-04-17T15:47:57+09:00	INFO	Starting Controller	{"controller": "accord-inventory-application", "controllerGroup": "argoproj.io", "controllerKind": "Application"}
2026-04-17T15:47:57+09:00	INFO	Starting workers	{"controller": "accord-inventory-application", "controllerGroup": "argoproj.io", "controllerKind": "Application", "worker count": 1}
2026-04-17T15:47:57+09:00	INFO	Enqueued inventory export	{"controller": "accord-inventory-application", "controllerGroup": "argoproj.io", "controllerKind": "Application", "Application": {"name":"test-app","namespace":"default"}, "namespace": "default", "name": "test-app", "reconcileID": "47e78b87-f3e6-4bca-9861-dddbe01685cb", "kind": "Application", "namespace": "default", "name": "test-app", "path": "inventory/applications/default/test-app.yaml"}
2026-04-17T15:48:08+09:00	INFO	git-batch-worker	Published inventory Git batch	{"files": 1}

## 2. 중복 깃 확인
2026-04-17T15:49:35+09:00	INFO	Enqueued inventory export	{"controller": "accord-inventory-application", "controllerGroup": "argoproj.io", "controllerKind": "Application", "Application": {"name":"test-app","namespace":"default"}, "namespace": "default", "name": "test-app", "reconcileID": "c32c6ae5-58ce-4fb8-b557-812c53ea86ef", "kind": "Application", "namespace": "default", "name": "test-app", "path": "inventory/applications/default/test-app.yaml"}
2026-04-17T15:49:45+09:00	DEBUG	git-batch-worker	Git export batch produced no diff; skipping commit