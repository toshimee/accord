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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"accord/internal/configmapmaterial"
)

// Reconciler implements inventory-controller export triggers for labeled ConfigMaps.
type Reconciler struct {
	client.Client
	Cache *HashCache
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			r.Cache.Delete(objectKey(req.Namespace, req.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	if cm.Labels == nil || cm.Labels[configmapmaterial.InventoryLabelKey] != configmapmaterial.InventoryLabelValue {
		return ctrl.Result{}, nil
	}

	h, err := configmapmaterial.MaterialSHA256Hex(&cm)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to compute material hash: %w", err)
	}

	key := objectKey(req.Namespace, req.Name)

	if ann, ok := cm.Annotations[configmapmaterial.SyncContentHashAnnotationKey]; ok && ann == h {
		log.V(1).Info("Ignored inventory reconcile; material hash matches sync annotation",
			"namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	if prev, ok := r.Cache.Get(key); ok && prev == h {
		log.V(1).Info("Ignored inventory reconcile due to matching in-process hash cache",
			"namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	r.Cache.Set(key, h)
	log.Info("Recorded inventory hash for ConfigMap",
		"namespace", req.Namespace, "name", req.Name, "hash", h)
	return ctrl.Result{}, nil
}

// SetupWithManager registers the inventory controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			lbl := obj.GetLabels()
			return lbl != nil && lbl[configmapmaterial.InventoryLabelKey] == configmapmaterial.InventoryLabelValue
		})).
		Named("accord-inventory").
		Complete(r)
}
