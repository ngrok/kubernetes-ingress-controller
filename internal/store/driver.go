package store

import (
	"context"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The name of the ingress controller which is uses to match on ingress classes
const controllerName = "k8s.ngrok.com/ingress-controller" // TODO:(initial-store) Let the user configure this

// Driver maintains the store of information, can derive new information from the store, and can
// synchronize the desired state of the store to the actual state of the cluster.
type Driver struct {
	Store       Storer
	cacheStores CacheStores
	log         logr.Logger
}

// NewDriver creates a new driver with a basic logger and cache store setup
func NewDriver(logger logr.Logger) *Driver {
	cacheStores := NewCacheStores(logger)
	s := New(cacheStores, controllerName, logger)
	return &Driver{
		Store:       s,
		cacheStores: cacheStores,
		log:         logger,
	}
}

// Seed fetches all the upfront information the driver needs to operate
// It needs to be seeded fully before it can be used to make calculations otherwise
// each calculation will be based on an incomplete state of the world. It currently relies on:
// - Ingresses
// - IngressClasses
// - Secrets // TODO:(initial-store)
// - Domains // TODO:(initial-store)
// - Edges // TODO:(initial-store)
// - other ngrok ones? Anything the ingress controller watches or creates basically
func (d *Driver) Seed(ctx context.Context, c client.Reader) error {
	ingresses := &netv1.IngressList{}
	if err := c.List(ctx, ingresses); err != nil {
		return err
	}

	ingressClasses := &netv1.IngressClassList{}
	if err := c.List(ctx, ingressClasses); err != nil {
		return err
	}

	for _, ing := range ingresses.Items {
		if err := d.Update(&ing); err != nil {
			return err
		}
	}

	for _, ingClass := range ingressClasses.Items {
		if err := d.Update(&ingClass); err != nil {
			return err
		}
	}

	return nil
}

// Delete is a simple proxy to the cacheStores Delete method
func (d *Driver) Delete(obj runtime.Object) error {
	return d.cacheStores.Delete(obj)
}

// Delete an ingress object given the NamespacedName
// Takes a namespacedName string as a parameter and
// deletes the ingress object from the cacheStores map
func (d *Driver) DeleteIngress(n types.NamespacedName) error {
	ingress := &netv1.Ingress{}
	// set NamespacedName on the ingress object
	ingress.SetNamespace(n.Namespace)
	ingress.SetName(n.Name)
	return d.cacheStores.Delete(ingress)
}

// Update is a simple proxy to the cacheStores Add method
// An add for an object with the same key thats already present is just an update
func (d *Driver) Update(obj runtime.Object) error {
	// we do a deep copy of the object here so that the caller can continue to use
	// the original object in a threadsafe manner.
	return d.cacheStores.Add(obj.DeepCopyObject())
}
