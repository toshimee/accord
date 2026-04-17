/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inventory

import (
	"testing"
)

// TDD: noisy YAML (system/status fields) and minimal YAML must yield the same SHA-256
// after normalization so inventory can treat them as the same material object.

func TestMaterialHashFromNormalizedYAML_noisyAndMinimal_ConfigMap_sameSHA256(t *testing.T) {
	noisy := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  namespace: ns1
  uid: 6c9fcd48-fc45-11ef-9c55-0242ac120002
  resourceVersion: "4242"
  creationTimestamp: "2024-01-02T15:04:05Z"
  generation: 7
  managedFields:
    - apiVersion: v1
      fieldsType: FieldsV1
      manager: kube-controller-manager
      operation: Update
  labels:
    app: accord
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{"apiVersion":"v1","kind":"ConfigMap"}'
data:
  foo: bar
`
	minimal := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  namespace: ns1
  labels:
    app: accord
data:
  foo: bar
`

	hNoisy, err := MaterialHashFromNormalizedYAML([]byte(noisy))
	if err != nil {
		t.Fatalf("MaterialHashFromNormalizedYAML(noisy): %v", err)
	}
	hMinimal, err := MaterialHashFromNormalizedYAML([]byte(minimal))
	if err != nil {
		t.Fatalf("MaterialHashFromNormalizedYAML(minimal): %v", err)
	}
	if hNoisy != hMinimal {
		t.Fatalf("expected identical SHA-256 after normalization; noisy=%s minimal=%s", hNoisy, hMinimal)
	}
	if len(hNoisy) != 64 {
		t.Fatalf("expected hex-encoded SHA-256 (64 chars), got len=%d", len(hNoisy))
	}
}

func TestMaterialHashFromNormalizedYAML_noisyAndMinimal_Pod_sameSHA256(t *testing.T) {
	noisy := `
apiVersion: v1
kind: Pod
metadata:
  name: p1
  namespace: prod
  uid: aaaabbbb-cccc-dddd-eeee-ffffffffffff
  resourceVersion: "99"
  creationTimestamp: "2025-06-01T12:00:00Z"
  managedFields: []
spec:
  containers:
    - name: main
      image: nginx:1.27
      ports:
        - containerPort: 80
status:
  phase: Running
  podIP: 10.0.0.1
  startTime: "2025-06-01T12:00:01Z"
  conditions: []
`
	minimal := `
apiVersion: v1
kind: Pod
metadata:
  name: p1
  namespace: prod
spec:
  containers:
    - name: main
      image: nginx:1.27
      ports:
        - containerPort: 80
`

	hNoisy, err := MaterialHashFromNormalizedYAML([]byte(noisy))
	if err != nil {
		t.Fatalf("MaterialHashFromNormalizedYAML(noisy pod): %v", err)
	}
	hMinimal, err := MaterialHashFromNormalizedYAML([]byte(minimal))
	if err != nil {
		t.Fatalf("MaterialHashFromNormalizedYAML(minimal pod): %v", err)
	}
	if hNoisy != hMinimal {
		t.Fatalf("expected identical SHA-256 for pod; noisy=%s minimal=%s", hNoisy, hMinimal)
	}
}

func TestMaterialHashFromNormalizedYAML_differentContent_differentSHA256(t *testing.T) {
	a := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: x
  namespace: n
data:
  k: one
`
	b := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: x
  namespace: n
  uid: will-be-stripped
data:
  k: two
`
	ha, err := MaterialHashFromNormalizedYAML([]byte(a))
	if err != nil {
		t.Fatal(err)
	}
	hb, err := MaterialHashFromNormalizedYAML([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	if ha == hb {
		t.Fatalf("expected different hashes for different data, both were %s", ha)
	}
}

func TestMaterialHashFromNormalizedYAML_syncContentHashAnnotationIgnored(t *testing.T) {
	base := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: x
  namespace: n
data:
  k: v
`
	withHash := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: x
  namespace: n
  annotations:
    accord.io/sync-content-hash: deadbeef0000
data:
  k: v
`
	h1, err := MaterialHashFromNormalizedYAML([]byte(base))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := MaterialHashFromNormalizedYAML([]byte(withHash))
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("material hash must ignore sync stamp: %s vs %s", h1, h2)
	}
}

func TestMaterialHashFromNormalizedYAML_firstDocumentOnly(t *testing.T) {
	multi := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: only
  namespace: n
data:
  a: "1"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ignored
  namespace: n
data:
  b: "2"
`
	hMulti, err := MaterialHashFromNormalizedYAML([]byte(multi))
	if err != nil {
		t.Fatal(err)
	}
	single := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: only
  namespace: n
data:
  a: "1"
`
	hSingle, err := MaterialHashFromNormalizedYAML([]byte(single))
	if err != nil {
		t.Fatal(err)
	}
	if hMulti != hSingle {
		t.Fatalf("expected first document only to be hashed; multi=%s single=%s", hMulti, hSingle)
	}
}
