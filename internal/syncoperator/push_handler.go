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

package syncoperator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"accord/internal/configmapmaterial"
)

// WebhookPath is the HTTP path for Git push style sync requests.
const WebhookPath = "/accord/v1/sync/push"

// PushRequest is the JSON body accepted by the sync webhook.
type PushRequest struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Data      map[string]string `json:"data"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// PushHandler applies ConfigMap material from a webhook payload.
type PushHandler struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;create;update;patch

func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := logf.FromContext(r.Context())
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Namespace == "" || req.Name == "" {
		http.Error(w, "namespace and name are required", http.StatusBadRequest)
		return
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels: map[string]string{
				configmapmaterial.InventoryLabelKey: configmapmaterial.InventoryLabelValue,
			},
		},
		Data: req.Data,
	}
	for k, v := range req.Labels {
		if desired.Labels == nil {
			desired.Labels = map[string]string{}
		}
		desired.Labels[k] = v
	}

	hDesired, err := configmapmaterial.MaterialSHA256Hex(desired)
	if err != nil {
		log.Error(err, "Failed to canonicalize sync payload")
		http.Error(w, "failed to canonicalize payload", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	var existing corev1.ConfigMap
	getErr := h.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, &existing)
	switch {
	case getErr == nil:
		hExisting, err := configmapmaterial.MaterialSHA256Hex(&existing)
		if err != nil {
			log.Error(err, "Failed to hash existing ConfigMap")
			http.Error(w, "failed to hash existing object", http.StatusInternalServerError)
			return
		}
		if hExisting == hDesired {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"skipped","reason":"hash_match"}`))
			return
		}
		existing.Labels = desired.Labels
		existing.Data = desired.Data
		if err := h.Client.Update(ctx, &existing); err != nil {
			log.Error(err, "Failed to update ConfigMap")
			http.Error(w, fmt.Sprintf("failed to update ConfigMap: %v", err), http.StatusInternalServerError)
			return
		}
	case apierrors.IsNotFound(getErr):
		if err := h.Client.Create(ctx, desired); err != nil {
			log.Error(err, "Failed to create ConfigMap")
			http.Error(w, fmt.Sprintf("failed to create ConfigMap: %v", err), http.StatusInternalServerError)
			return
		}
	default:
		log.Error(getErr, "Failed to get ConfigMap")
		http.Error(w, fmt.Sprintf("failed to get ConfigMap: %v", getErr), http.StatusInternalServerError)
		return
	}

	if err := stampSyncContentHash(ctx, h.Client, req.Namespace, req.Name, hDesired); err != nil {
		log.Error(err, "Failed to stamp sync content hash annotation")
		http.Error(w, fmt.Sprintf("failed to annotate ConfigMap: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"applied"}`))
}

func stampSyncContentHash(ctx context.Context, c client.Client, namespace, name, hash string) error {
	var cm corev1.ConfigMap
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &cm); err != nil {
		return fmt.Errorf("failed to get ConfigMap for annotation: %w", err)
	}
	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	cm.Annotations[configmapmaterial.SyncContentHashAnnotationKey] = hash
	if err := c.Update(ctx, &cm); err != nil {
		return fmt.Errorf("failed to update ConfigMap annotations: %w", err)
	}
	return nil
}
