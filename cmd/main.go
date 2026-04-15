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

package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	opsv1alpha1 "accord/api/v1alpha1"
	"accord/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(opsv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// Development note: inventory-controller and sync-operator are intentionally wired in this
// single binary to iterate quickly. They share an in-memory hash cache to break sync loops.

const (
	inventoryLabelKey   = "accord.io/inventory"
	inventoryLabelValue = "true"
	syncWebhookPath     = "/accord/v1/sync/push"
)

// hashCache stores the last known canonical SHA-256 for a namespaced object (namespace/name).
type hashCache struct {
	mu sync.RWMutex
	m  map[string]string
}

func newHashCache() *hashCache {
	return &hashCache{m: make(map[string]string)}
}

func objectKey(namespace, name string) string {
	return namespace + "/" + name
}

func (c *hashCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *hashCache) Set(key, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = hash
}

func (c *hashCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

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

func canonicalConfigMap(cm *corev1.ConfigMap) ([]byte, error) {
	var c configMapCanonical
	c.APIVersion = "v1"
	c.Kind = "ConfigMap"
	c.Metadata.Namespace = cm.Namespace
	c.Metadata.Name = cm.Name
	if len(cm.Labels) > 0 {
		c.Metadata.Labels = cm.Labels
	}
	if len(cm.Annotations) > 0 {
		c.Metadata.Annotations = cm.Annotations
	}
	if len(cm.Data) > 0 {
		c.Data = cm.Data
	}
	return json.Marshal(c)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// inventoryReconciler watches labeled ConfigMaps, normalizes them to a canonical JSON form,
// hashes the payload, and uses the shared cache to ignore reconciles that match the last
// known state (for example right after sync-operator applied the same manifest).
type inventoryReconciler struct {
	client.Client
	cache *hashCache
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch

func (r *inventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			r.cache.Delete(objectKey(req.Namespace, req.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	if cm.Labels == nil || cm.Labels[inventoryLabelKey] != inventoryLabelValue {
		return ctrl.Result{}, nil
	}

	canon, err := canonicalConfigMap(&cm)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build canonical ConfigMap JSON: %w", err)
	}
	h := sha256Hex(canon)
	key := objectKey(req.Namespace, req.Name)
	if prev, ok := r.cache.Get(key); ok && prev == h {
		log.V(1).Info("Ignored inventory reconcile due to matching hash",
			"namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	r.cache.Set(key, h)
	log.Info("Recorded inventory hash for ConfigMap",
		"namespace", req.Namespace, "name", req.Name, "hash", h)
	return ctrl.Result{}, nil
}

func (r *inventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			lbl := obj.GetLabels()
			return lbl != nil && lbl[inventoryLabelKey] == inventoryLabelValue
		})).
		Named("accord-inventory").
		Complete(r)
}

type syncPushRequest struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Data      map[string]string `json:"data"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// syncPushHandler receives Git (or CI) push notifications and applies ConfigMap data when the
// canonical hash differs from the shared cache (idempotent apply).
type syncPushHandler struct {
	client.Client
	cache *hashCache
}

func (h *syncPushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req syncPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Namespace == "" || req.Name == "" {
		http.Error(w, "namespace and name are required", http.StatusBadRequest)
		return
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels: map[string]string{
				inventoryLabelKey: inventoryLabelValue,
			},
		},
		Data: req.Data,
	}
	if len(req.Labels) > 0 {
		for k, v := range req.Labels {
			cm.Labels[k] = v
		}
	}

	canon, err := canonicalConfigMap(cm)
	if err != nil {
		http.Error(w, "failed to canonicalize payload", http.StatusInternalServerError)
		return
	}
	payloadHash := sha256Hex(canon)
	key := objectKey(req.Namespace, req.Name)
	if prev, ok := h.cache.Get(key); ok && prev == payloadHash {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"skipped","reason":"hash_match"}`))
		return
	}

	ctx := r.Context()
	var existing corev1.ConfigMap
	err = h.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, &existing)
	switch {
	case apierrors.IsNotFound(err):
		if err := h.Create(ctx, cm); err != nil {
			http.Error(w, fmt.Sprintf("failed to create ConfigMap: %v", err), http.StatusInternalServerError)
			return
		}
	case err != nil:
		http.Error(w, fmt.Sprintf("failed to get ConfigMap: %v", err), http.StatusInternalServerError)
		return
	default:
		existing.Labels = cm.Labels
		existing.Data = cm.Data
		if err := h.Update(ctx, &existing); err != nil {
			http.Error(w, fmt.Sprintf("failed to update ConfigMap: %v", err), http.StatusInternalServerError)
			return
		}
	}

	h.cache.Set(key, payloadHash)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"applied"}`))
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b858da39.accord.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	if err := (&controller.MirrorUpgradeRequestReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "MirrorUpgradeRequest")
		os.Exit(1)
	}

	sharedHashCache := newHashCache()
	if err := (&inventoryReconciler{
		Client: mgr.GetClient(),
		cache:  sharedHashCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "accord-inventory")
		os.Exit(1)
	}

	mgr.GetWebhookServer().Register(syncWebhookPath, &syncPushHandler{
		Client: mgr.GetClient(),
		cache:  sharedHashCache,
	})
	setupLog.Info("Registered sync webhook endpoint", "path", syncWebhookPath)

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
