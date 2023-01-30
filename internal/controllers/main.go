package controllers

import (
	"context"
	"fmt"
	"strconv"

	netv1 "k8s.io/api/networking/v1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getIngress(ctx context.Context, c client.Client, namespacedName types.NamespacedName) (*netv1.Ingress, error) {
	ingress := &netv1.Ingress{}
	if err := c.Get(ctx, namespacedName, ingress); err != nil {
		return nil, err
	}

	return ingress, nil
}

// Checks the ingress object to make sure its using a the limited set of configuration options
// that we support. Returns an error if the ingress object is not valid
func validateIngress(ctx context.Context, ingress *netv1.Ingress) error {
	if len(ingress.Spec.Rules) > 1 {
		return fmt.Errorf("A maximum of one rule is required to be set")
	}
	if len(ingress.Spec.Rules) == 0 {
		return fmt.Errorf("At least one rule is required to be set")
	}
	if ingress.Spec.Rules[0].Host == "" {
		return fmt.Errorf("A host is required to be set")
	}
	for _, path := range ingress.Spec.Rules[0].HTTP.Paths {
		if path.Backend.Resource != nil {
			return fmt.Errorf("Resource backends are not supported")
		}
	}

	return nil
}

// Generates a labels map for matching ngrok Routes to Agent Tunnels
func backendToLabelMap(backend netv1.IngressBackend, namespace string) map[string]string {
	return map[string]string{
		"k8s.ngrok.com/namespace": namespace,
		"k8s.ngrok.com/service":   backend.Service.Name,
		"k8s.ngrok.com/port":      strconv.Itoa(int(backend.Service.Port.Number)),
	}
}
