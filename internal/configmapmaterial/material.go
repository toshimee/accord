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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// SyncContentHashAnnotationKey is written by sync-operator after a successful apply.
// It stores the material SHA-256 so inventory-controller can detect self-originated applies.
const SyncContentHashAnnotationKey = "accord.io/sync-content-hash"

// InventoryLabelKey marks ConfigMaps exported by inventory-controller.
const InventoryLabelKey = "accord.io/inventory"

// InventoryLabelValue is the label value that opts a ConfigMap into inventory export.
const InventoryLabelValue = "true"

type configMapCanonical struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Namespace   string            `json:"namespace"`
		Name        string            `json:"name"`
		Labels      map[string]string `json:"labels,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
	} `json:"metadata"`
	Data map[string]string `json:"data,omitempty"`
}

// materialView returns a deep copy with sync content-hash annotation removed so the digest
// reflects user/Git material only.
func materialView(cm *corev1.ConfigMap) *corev1.ConfigMap {
	out := cm.DeepCopy()
	if out.Annotations != nil {
		delete(out.Annotations, SyncContentHashAnnotationKey)
	}
	return out
}

// MaterialCanonicalJSON builds deterministic JSON for the material fields of a ConfigMap.
func MaterialCanonicalJSON(cm *corev1.ConfigMap) ([]byte, error) {
	v := materialView(cm)
	var c configMapCanonical
	c.APIVersion = "v1"
	c.Kind = "ConfigMap"
	c.Metadata.Namespace = v.Namespace
	c.Metadata.Name = v.Name
	if len(v.Labels) > 0 {
		c.Metadata.Labels = v.Labels
	}
	if len(v.Annotations) > 0 {
		c.Metadata.Annotations = v.Annotations
	}
	if len(v.Data) > 0 {
		c.Data = v.Data
	}
	return json.Marshal(c)
}

// MaterialSHA256Hex returns the hex-encoded SHA-256 of MaterialCanonicalJSON(cm).
func MaterialSHA256Hex(cm *corev1.ConfigMap) (string, error) {
	b, err := MaterialCanonicalJSON(cm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal canonical ConfigMap: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
