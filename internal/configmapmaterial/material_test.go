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

package configmapmaterial

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMaterialSHA256Hex_stableAndStripsSyncAnnotation(t *testing.T) {
	base := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-a",
			Name:      "cm-1",
			Labels: map[string]string{
				"app": "demo",
			},
			Annotations: map[string]string{
				"note": "x",
			},
		},
		Data: map[string]string{"k": "v"},
	}

	h0, err := MaterialSHA256Hex(base)
	if err != nil {
		t.Fatalf("MaterialSHA256Hex: %v", err)
	}

	withStamp := base.DeepCopy()
	if withStamp.Annotations == nil {
		withStamp.Annotations = map[string]string{}
	}
	withStamp.Annotations[SyncContentHashAnnotationKey] = "deadbeef"

	h1, err := MaterialSHA256Hex(withStamp)
	if err != nil {
		t.Fatalf("MaterialSHA256Hex stamped: %v", err)
	}
	if h1 != h0 {
		t.Fatalf("content hash annotation must not change material digest: %s vs %s", h0, h1)
	}

	otherNS := base.DeepCopy()
	otherNS.Namespace = "ns-b"
	h2, err := MaterialSHA256Hex(otherNS)
	if err != nil {
		t.Fatalf("MaterialSHA256Hex other ns: %v", err)
	}
	if h2 == h0 {
		t.Fatalf("expected different hash for different namespace")
	}
}

func TestMaterialSHA256Hex_dataChangeChangesHash(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "c"},
		Data:       map[string]string{"a": "1"},
	}
	h1, err := MaterialSHA256Hex(cm)
	if err != nil {
		t.Fatal(err)
	}
	cm.Data["a"] = "2"
	h2, err := MaterialSHA256Hex(cm)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatalf("expected hash to change when data changes")
	}
}
