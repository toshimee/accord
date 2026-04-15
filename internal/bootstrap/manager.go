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

package bootstrap

import (
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// ManagerInputs carries parsed flags and scheme for building a controller-runtime manager.
type ManagerInputs struct {
	Scheme               *runtime.Scheme
	LeaderElectionID     string
	MetricsAddr          string
	ProbeAddr            string
	EnableLeaderElection bool
	SecureMetrics        bool
	EnableHTTP2          bool
	MetricsCertPath      string
	MetricsCertName      string
	MetricsCertKey       string
	WebhookCertPath      string
	WebhookCertName      string
	WebhookCertKey       string
}

// NewManager constructs a ctrl.Manager from ManagerInputs (call after flag.Parse and ctrl.SetLogger).
func NewManager(log logr.Logger, in ManagerInputs) (ctrl.Manager, error) {
	var tlsOpts []func(*tls.Config)
	disableHTTP2 := func(c *tls.Config) {
		log.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}
	if !in.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}
	if len(in.WebhookCertPath) > 0 {
		log.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", in.WebhookCertPath, "webhook-cert-name", in.WebhookCertName, "webhook-cert-key", in.WebhookCertKey)
		webhookServerOptions.CertDir = in.WebhookCertPath
		webhookServerOptions.CertName = in.WebhookCertName
		webhookServerOptions.KeyName = in.WebhookCertKey
	}
	webhookServer := webhook.NewServer(webhookServerOptions)

	metricsServerOptions := metricsserver.Options{
		BindAddress:   in.MetricsAddr,
		SecureServing: in.SecureMetrics,
		TLSOpts:       tlsOpts,
	}
	if in.SecureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}
	if len(in.MetricsCertPath) > 0 {
		log.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", in.MetricsCertPath, "metrics-cert-name", in.MetricsCertName, "metrics-cert-key", in.MetricsCertKey)
		metricsServerOptions.CertDir = in.MetricsCertPath
		metricsServerOptions.CertName = in.MetricsCertName
		metricsServerOptions.KeyName = in.MetricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 in.Scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: in.ProbeAddr,
		LeaderElection:         in.EnableLeaderElection,
		LeaderElectionID:       in.LeaderElectionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}
	return mgr, nil
}
