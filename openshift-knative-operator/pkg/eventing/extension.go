package eventing

import (
	"context"
	"fmt"
	"os"

	mf "github.com/manifestival/manifestival"
	"github.com/openshift-knative/serverless-operator/openshift-knative-operator/pkg/common"
	"github.com/openshift-knative/serverless-operator/openshift-knative-operator/pkg/monitoring"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"knative.dev/operator/pkg/apis/operator/v1alpha1"
	operator "knative.dev/operator/pkg/reconciler/common"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
)

const requiredNsEnvName = "REQUIRED_EVENTING_NAMESPACE"

// NewExtension creates a new extension for a Knative Eventing controller.
func NewExtension(ctx context.Context) operator.Extension {
	return &extension{
		kubeclient: kubeclient.Get(ctx),
	}
}

type extension struct {
	kubeclient kubernetes.Interface
}

func (e *extension) Manifests(ke v1alpha1.KComponent) ([]mf.Manifest, error) {
	return monitoring.GetEventingMonitoringPlatformManifests(ke)
}

func (e *extension) Transformers(ke v1alpha1.KComponent) []mf.Transformer {
	return monitoring.GetEventingTransformers(ke)
}

func (e *extension) Reconcile(ctx context.Context, comp v1alpha1.KComponent) error {
	ke := comp.(*v1alpha1.KnativeEventing)

	requiredNs := os.Getenv(requiredNsEnvName)
	if requiredNs != "" && ke.Namespace != requiredNs {
		ke.Status.MarkInstallFailed(fmt.Sprintf("Knative Eventing must be installed into the namespace %q", requiredNs))
		return controller.NewPermanentError(fmt.Errorf("deployed Knative Serving into unsupported namespace %q", ke.Namespace))
	}

	// Override images.
	// TODO(SRVCOM-1069): Rethink overriding behavior and/or error surfacing.
	images := common.ImageMapFromEnvironment(os.Environ())
	ke.Spec.Registry.Override = images
	ke.Spec.Registry.Default = images["default"]

	// Ensure webhook has 1G of memory.
	common.EnsureContainerMemoryLimit(&ke.Spec.CommonSpec, "eventing-webhook", resource.MustParse("1024Mi"))

	// SRVKE-500: Ensure we set the SinkBindingSelectionMode to inclusion
	if ke.Spec.SinkBindingSelectionMode == "" {
		ke.Spec.SinkBindingSelectionMode = "inclusion"
	}

	if err := monitoring.ReconcileMonitoringForEventing(ctx, e.kubeclient, ke); err != nil {
		return err
	}
	return nil
}

func (e *extension) Finalize(context.Context, v1alpha1.KComponent) error {
	return nil
}
