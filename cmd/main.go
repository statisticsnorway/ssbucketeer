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

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/storage"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/iam/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/caarlos0/env/v11"

	"github.com/statisticsnorway/ssbucketeer/internal/controller"
	//+kubebuilder:scaffold:imports
)

type config struct {
	DaplaGroupSaProjectId string `env:"DAPLA_GROUP_SA_PROJECT_ID,required,notEmpty"`
	TeamsFolderNumber     string `env:"TEAMS_FOLDER_NUMBER,required,notEmpty"`
	Stage                 string `env:"STAGE,required,notEmpty"`
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
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
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "3ab6dba2.ssb.no",
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
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := env.ParseAsWithOptions[config](env.Options{
		Prefix: "SSBUCKETEER_",
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

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		(&controller.StatefulsetMutator{
			Client:            mgr.GetClient(),
			Decoder:           admission.NewDecoder(mgr.GetScheme()),
			Storage:           storageClient,
			Projects:          projectsClient,
			Folders:           foldersClient,
			TeamsFolderNumber: cfg.TeamsFolderNumber,
			Stage:             cfg.Stage,
		}).SetupWithManager(mgr)

		if err = (&controller.ServiceAccountValidator{
			Auth: gAuth,
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
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceAccount")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
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
