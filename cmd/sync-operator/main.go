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
	"flag"
	"net/http"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"accord/internal/bootstrap"
	"accord/internal/config"
	"accord/internal/sync"
)

var setupLog = ctrl.Log.WithName("setup")

func main() {
	cfg, err := config.LoadInventoryControllerConfig()
	if err != nil {
		setupLog.Error(err, "Failed to load configuration (shared env schema for Git token)")
		os.Exit(1)
	}

	scheme := bootstrap.NewInventoryScheme()

	var metricsAddr, probeAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection, secureMetrics, enableHTTP2 bool

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
	zopts := zap.Options{Development: true}
	zopts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zopts)))

	mgr, err := bootstrap.NewManager(setupLog, bootstrap.ManagerInputs{
		Scheme:               scheme,
		LeaderElectionID:     "accord-sync-operator.accord.io",
		MetricsAddr:          metricsAddr,
		ProbeAddr:            probeAddr,
		EnableLeaderElection: enableLeaderElection,
		SecureMetrics:        secureMetrics,
		EnableHTTP2:          enableHTTP2,
		MetricsCertPath:      metricsCertPath,
		MetricsCertName:      metricsCertName,
		MetricsCertKey:       metricsCertKey,
		WebhookCertPath:      webhookCertPath,
		WebhookCertName:      webhookCertName,
		WebhookCertKey:       webhookCertKey,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 2 * time.Minute}
	h := &sync.WebhookHandler{
		K8s:         mgr.GetClient(),
		HTTPClient:  httpClient,
		GitHubToken: cfg.GitAccessToken,
	}
	mgr.GetWebhookServer().Register(sync.WebhookPath, h)
	setupLog.Info("Registered sync-operator webhook", "path", sync.WebhookPath)

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting sync-operator manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
