/*
Copyright 2024.

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
	"crypto/tls"
	"flag"
	"io"
	"net/http"
	"os"
	"reflect"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/storage"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/iam/v1"
	"gopkg.in/yaml.v3"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/caarlos0/env/v11"

	"github.com/statisticsnorway/ssbucketeer/internal/audit"
	"github.com/statisticsnorway/ssbucketeer/internal/controller"
	// +kubebuilder:scaffold:imports
)

type config struct {
	DaplaGroupSaProjectId string                          `env:"DAPLA_GROUP_SA_PROJECT_ID,required,notEmpty"`
	TeamsFolderNumber     string                          `env:"TEAMS_FOLDER_NUMBER,required,notEmpty"`
	Stage                 string                          `env:"STAGE,required,notEmpty"`
	IamProbeImage         string                          `env:"IAM_PROBE_IMAGE"`
	PrecreatorImage       *string                         `env:"PRECREATOR_IMAGE"`
	ADCGroupEnvName       string                          `env:"ADC_GROUP_ENV_NAME"`
	TeamGcpProjectEnvName string                          `env:"TEAM_GCP_PROJECT_ENV_NAME"`
	GroupConfigs          []controller.AccessGroupConfig  `env:"GROUP_CONFIG,required,notEmpty"`
	AuditSinks            []auditSink                     `env:"AUDIT_SINKS"`
	SharedBucketTemplate  controller.SharedBucketTemplate `env:"SHARED_BUCKET_TEMPLATE" envDefault:"ssb-{{.TeamName}}-data-delt-{{.BucketShortName}}-{{.Stage}}"`
}

type auditSink struct {
	Type   string            `yaml:"type"`
	Config map[string]string `yaml:"config"`
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
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
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/metrics/server
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
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
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
		LeaderElectionID:       "47649604.ssb.no",
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

	cfg, err := env.ParseAsWithOptions[config](env.Options{
		Prefix: "SSBUCKETEER_",
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf([]controller.AccessGroupConfig{}): yamlParser[[]controller.AccessGroupConfig],
			reflect.TypeOf([]auditSink{}):                    yamlParser[[]auditSink],
		},
	})
	if err != nil {
		setupLog.Error(err, "failed to parse environment variables")
		os.Exit(1)
	}

	ctx := context.Background()

	gkeProjectId, err := getGKEProjectId()
	if err != nil {
		setupLog.Error(err, "failed to get GKE project ID")
		os.Exit(1)
	}

	ciService, err := cloudidentity.NewService(ctx)
	if err != nil {
		setupLog.Error(err, "failed to create Cloud Identity service")
		os.Exit(1)
	}
	gAuth := &controller.GoogleAuther{Ci: ciService}

	iamService, err := iam.NewService(ctx)
	if err != nil {
		setupLog.Error(err, "failed to set up IAM service")
		os.Exit(1)
	}

	projectsClient, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		setupLog.Error(err, "failed to set up projects client")
		os.Exit(1)
	}

	foldersClient, err := resourcemanager.NewFoldersClient(ctx)
	if err != nil {
		setupLog.Error(err, "failed to set up folders client")
		os.Exit(1)
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		setupLog.Error(err, "failed to set up storage client")
		os.Exit(1)
	}

	var sinks []audit.Sink
	for _, sink := range cfg.AuditSinks {
		switch sink.Type {
		case "cloudlogging":
			cl, err := audit.NewCloudLoggingSink(ctx, sink.Config)
			if err != nil {
				setupLog.Error(err, "failed to configure cloud logging sink", "sink.Config", sink.Config)
				os.Exit(1)
			}
			sinks = append(sinks, cl)
		case "cloudstorage":
			cs, err := audit.NewCloudStorageSink(ctx, sink.Config)
			if err != nil {
				setupLog.Error(err, "failed to configure cloud storage sink", "sink.Config", sink.Config)
				os.Exit(1)
			}
			sinks = append(sinks, cs)
		}
	}
	auditRouter := audit.NewRouter(sinks...)
	defer func() {
		if err := auditRouter.FlushAll(); err != nil {
			ctrl.Log.Error(err, "error flushing audit log sinks")
		}
	}()

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		(&controller.StatefulsetMutator{
			Client:                mgr.GetClient(),
			Decoder:               admission.NewDecoder(mgr.GetScheme()),
			Storage:               storageClient,
			Projects:              projectsClient,
			Folders:               foldersClient,
			TeamsFolderNumber:     cfg.TeamsFolderNumber,
			Stage:                 cfg.Stage,
			IamProbeImage:         cfg.IamProbeImage,
			PrecreatorImage:       cfg.PrecreatorImage,
			ADCGroupEnvName:       cfg.ADCGroupEnvName,
			TeamGcpProjectEnvName: cfg.TeamGcpProjectEnvName,
			GroupConfigs:          cfg.GroupConfigs,
			SharedBucketTemplate:  cfg.SharedBucketTemplate,
		}).SetupWithManager(mgr)

		if err = (&controller.ServiceAccountValidator{
			Auth:         gAuth,
			GroupConfigs: cfg.GroupConfigs,
			Audit:        auditRouter,
			Stage:        cfg.Stage,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to register webhook", "webhook", "ServiceAccount")
			os.Exit(1)
		}
	}

	if err = (&controller.ServiceAccountReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		DaplaGroupSaProject: cfg.DaplaGroupSaProjectId,
		ClusterProjectId:    gkeProjectId,
		Iam:                 iamService,
		Auth:                gAuth,
		GroupConfigs:        cfg.GroupConfigs,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceAccount")
		os.Exit(1)
	}
	if err = (&controller.JobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "statefulset")
		os.Exit(1)
	}
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

func getGKEProjectId() (string, error) {
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// yamlParser is a helper function for parsing environment variables as YAML strings.
//
// To parse an environment variable as a YAML string, simply add an entry to the
// env.Option.FuncMap when parsing:
//
//	  env.ParseWithOptions(&config, env.Options{
//			FuncMap: map[reflect.Type]env.ParserFunc{
//				reflect.TypeOf(CustomType{}): yamlParser[CustomType],
//			},
//	 })
func yamlParser[T any](v string) (any, error) {
	var out T
	if err := yaml.Unmarshal([]byte(v), &out); err != nil {
		return nil, err
	}
	return out, nil
}
