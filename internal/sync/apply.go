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

package sync

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigyaml "sigs.k8s.io/yaml"

	"accord/internal/configmapmaterial"
	"accord/internal/inventory"
)

// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;create;patch
// +kubebuilder:rbac:groups=argoproj.io,resources=applicationsets,verbs=get;create;patch

const fieldOwner = "accord-sync-operator"

// ApplyInventoryYAML applies Git YAML to the cluster using server-side apply after injecting
// accord.io/sync-content-hash (material hash from inventory normalization) per docs/phase1-sync.md.
func ApplyInventoryYAML(ctx context.Context, c client.Client, gitYAML []byte) error {
	incomingHash, err := inventory.MaterialHashFromNormalizedYAML(gitYAML)
	if err != nil {
		return fmt.Errorf("compute material hash from Git YAML: %w", err)
	}

	doc := inventory.FirstYAMLDocument(gitYAML)
	jsonBytes, err := sigyaml.YAMLToJSON(doc)
	if err != nil {
		return fmt.Errorf("convert yaml to json: %w", err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(jsonBytes); err != nil {
		return fmt.Errorf("parse unstructured: %w", err)
	}

	ann := obj.GetAnnotations()
	next := make(map[string]string)
	for k, v := range ann {
		next[k] = v
	}
	next[configmapmaterial.SyncContentHashAnnotationKey] = incomingHash
	obj.SetAnnotations(next)

	if err := c.Patch(ctx, obj, client.Apply, client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return fmt.Errorf("server-side apply: %w", err)
	}
	return nil
}
