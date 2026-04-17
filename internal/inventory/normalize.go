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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// metadata keys removed before hashing (volatile / system-populated).
var stripMetadataKeys = []string{
	"uid",
	"resourceVersion",
	"creationTimestamp",
	"deletionTimestamp",
	"generation",
	"managedFields",
	"selfLink",
}

const lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// MaterialHashFromNormalizedYAML parses the first YAML document as a Kubernetes object,
// strips system-only fields (status, selected metadata keys, last-applied annotation),
// marshals the remainder to canonical JSON, and returns the hex-encoded SHA-256 digest.
func MaterialHashFromNormalizedYAML(src []byte) (string, error) {
	doc := firstYAMLDocument(src)
	jsonBytes, err := yaml.YAMLToJSON(doc)
	if err != nil {
		return "", fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return "", fmt.Errorf("failed to unmarshal object JSON: %w", err)
	}
	normalizeKubernetesObject(obj)
	out, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal normalized object: %w", err)
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:]), nil
}

func firstYAMLDocument(src []byte) []byte {
	s := strings.TrimSpace(string(src))
	if s == "" {
		return []byte{}
	}
	// Split on document separators; only the first document is considered.
	parts := strings.Split(s, "\n---")
	first := strings.TrimSpace(parts[0])
	return []byte(first)
}

func normalizeKubernetesObject(obj map[string]interface{}) {
	delete(obj, "status")

	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	for _, k := range stripMetadataKeys {
		delete(meta, k)
	}
	if ann, ok := meta["annotations"].(map[string]interface{}); ok {
		delete(ann, lastAppliedAnnotation)
		if len(ann) == 0 {
			delete(meta, "annotations")
		}
	}
	if len(meta) == 0 {
		delete(obj, "metadata")
	}
}

func normalizedMapFromRuntimeObject(obj runtime.Object) (map[string]interface{}, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime object: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal object JSON: %w", err)
	}
	normalizeKubernetesObject(m)
	return m, nil
}

// NormalizedYAMLAndMaterialHash returns normalized export YAML and the SHA-256 hex digest of the
// normalized canonical JSON (same normalization rules as MaterialHashFromNormalizedYAML).
func NormalizedYAMLAndMaterialHash(obj runtime.Object) ([]byte, string, error) {
	m, err := normalizedMapFromRuntimeObject(obj)
	if err != nil {
		return nil, "", err
	}
	j, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("marshal normalized map: %w", err)
	}
	sum := sha256.Sum256(j)
	h := hex.EncodeToString(sum[:])
	yb, err := yaml.JSONToYAML(j)
	if err != nil {
		return nil, "", fmt.Errorf("json to yaml: %w", err)
	}
	return yb, h, nil
}
