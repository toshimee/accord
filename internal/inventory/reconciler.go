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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"accord/internal/configmapmaterial"
	"accord/internal/git"
)

// ReconcileDeps is shared state for Argo inventory reconcilers (DI from main).
type ReconcileDeps struct {
	Client         client.Client
	Cache          *HashCache
	GitQueue       *git.BatchWorker
	ExportPathTmpl string
}

func (d *ReconcileDeps) handle(ctx context.Context, req ctrl.Request, obj client.Object, kind string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	yamlBytes, currentHash, err := NormalizedYAMLAndMaterialHash(obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("normalize and hash object: %w", err)
	}

	cacheKey := ExportObjectCacheKey(kind, req.Namespace, req.Name)

	if ann := obj.GetAnnotations(); ann != nil {
		if v, ok := ann[configmapmaterial.SyncContentHashAnnotationKey]; ok && v == currentHash {
			log.Info("Hash matched (annotation). Ignoring event to break loop",
				"kind", kind, "namespace", req.Namespace, "name", req.Name, "hash", currentHash)
			return ctrl.Result{}, nil
		}
	}

	if prev, ok := d.Cache.Get(cacheKey); ok && prev == currentHash {
		log.Info("Hash matched (cache). Ignoring event to break loop",
			"kind", kind, "namespace", req.Namespace, "name", req.Name, "hash", currentHash)
		return ctrl.Result{}, nil
	}

	d.Cache.Set(cacheKey, currentHash)

	relPath, err := RenderExportPath(d.ExportPathTmpl, kind, req.Namespace, req.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("render export path: %w", err)
	}

	d.GitQueue.Enqueue(git.ExportJob{Path: relPath, Content: yamlBytes})
	log.Info("Enqueued inventory export", "kind", kind, "namespace", req.Namespace, "name", req.Name, "path", relPath)
	return ctrl.Result{}, nil
}

// ApplicationReconciler watches Argo CD Applications (argoproj.io/v1alpha1).
type ApplicationReconciler struct {
	*ReconcileDeps
}

// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ApplicationGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, u); err != nil {
		if apierrors.IsNotFound(err) {
			r.Cache.Delete(ExportObjectCacheKey("Application", req.Namespace, req.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Application: %w", err)
	}
	return r.handle(ctx, req, u, "Application")
}

// SetupWithManager registers the Application controller.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	proto := &unstructured.Unstructured{}
	proto.SetGroupVersionKind(ApplicationGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(proto).
		Named("accord-inventory-application").
		Complete(r)
}

// ApplicationSetReconciler watches Argo CD ApplicationSets (argoproj.io/v1alpha1).
type ApplicationSetReconciler struct {
	*ReconcileDeps
}

// +kubebuilder:rbac:groups=argoproj.io,resources=applicationsets,verbs=get;list;watch

func (r *ApplicationSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ApplicationSetGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, u); err != nil {
		if apierrors.IsNotFound(err) {
			r.Cache.Delete(ExportObjectCacheKey("ApplicationSet", req.Namespace, req.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ApplicationSet: %w", err)
	}
	return r.handle(ctx, req, u, "ApplicationSet")
}

// SetupWithManager registers the ApplicationSet controller.
func (r *ApplicationSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	proto := &unstructured.Unstructured{}
	proto.SetGroupVersionKind(ApplicationSetGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(proto).
		Named("accord-inventory-applicationset").
		Complete(r)
}
