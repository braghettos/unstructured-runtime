// Command quickstart runs a minimal unstructured-runtime controller out-of-cluster:
// a no-op ExternalClient reconciling an arbitrary GVR through the builder.
//
// It only logs what the runtime asks it to do — safe to point at any cluster.
//
//	go run ./examples/quickstart \
//	  -group samples.krateo.io -version v1 -resource samples
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/krateo-platformops/unstructured-runtime/pkg/controller"
	"github.com/krateo-platformops/unstructured-runtime/pkg/controller/builder"
	"github.com/krateo-platformops/unstructured-runtime/pkg/logging"
	"github.com/krateo-platformops/unstructured-runtime/pkg/signals"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
)

// nopExternalClient satisfies controller.ExternalClient without touching any
// external system. Observe is a pure read by construction — it must never
// write mg (see docs/overview.md, "Observe must be side-effect-free").
type nopExternalClient struct {
	log logging.Logger
}

func (c *nopExternalClient) Observe(ctx context.Context, mg *unstructured.Unstructured) (controller.ExternalObservation, error) {
	c.log.Info("Observe", "name", mg.GetName(), "namespace", mg.GetNamespace())
	// Pretending the external resource exists and is up to date keeps this
	// demo side-effect-free end to end.
	return controller.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (c *nopExternalClient) Create(ctx context.Context, mg *unstructured.Unstructured) error {
	c.log.Info("Create", "name", mg.GetName(), "namespace", mg.GetNamespace())
	return nil
}

func (c *nopExternalClient) Update(ctx context.Context, mg *unstructured.Unstructured) error {
	c.log.Info("Update", "name", mg.GetName(), "namespace", mg.GetNamespace())
	return nil
}

func (c *nopExternalClient) Delete(ctx context.Context, mg *unstructured.Unstructured) error {
	c.log.Info("Delete", "name", mg.GetName(), "namespace", mg.GetNamespace())
	return nil
}

func main() {
	var group, version, resource, namespace string
	flag.StringVar(&group, "group", "samples.krateo.io", "API group of the resource to reconcile")
	flag.StringVar(&version, "version", "v1", "API version of the resource to reconcile")
	flag.StringVar(&resource, "resource", "samples", "plural resource name to reconcile")
	flag.StringVar(&namespace, "namespace", "", "namespace to watch (empty = all)")
	flag.Parse()

	log := logging.NewSlogLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	// Out-of-cluster config from the usual kubeconfig loading rules
	// (KUBECONFIG or ~/.kube/config). In-cluster, use builder.GetConfig().
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		log.Error(err, "cannot load kubeconfig")
		os.Exit(1)
	}

	ctrl, err := builder.Build(context.Background(), builder.Configuration{
		Config:       cfg,
		GVR:          schema.GroupVersionResource{Group: group, Version: version, Resource: resource},
		ProviderName: "quickstart-provider",
	},
		builder.WithLogger(log),
		builder.WithNamespace(namespace),
	)
	if err != nil {
		log.Error(err, "cannot build controller")
		os.Exit(1)
	}

	ctrl.SetExternalClient(&nopExternalClient{log: log})

	// Cancel the root context on SIGTERM/SIGINT; Run then drains in-flight
	// reconciles for up to the graceful-shutdown window (default 30s).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-signals.SetupSignalHandler()
		cancel()
	}()

	if err := ctrl.Run(ctx, 2); err != nil {
		log.Error(err, "controller exited with error")
		os.Exit(1)
	}
}
