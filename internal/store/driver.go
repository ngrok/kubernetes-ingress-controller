package store

import (
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type Driver struct {
	Store       Storer
	cacheStores CacheStores
}

func NewDriver(store Storer) *Driver {
	return &Driver{
		Store: store,
	}
}

func (d *Driver) Delete(obj runtime.Object) error {
	return d.cacheStores.Delete(obj)
}

func (d *Driver) Add(obj runtime.Object) error {
	return d.cacheStores.Delete(obj)
}

// Delete an ingress object given the NamespacedName
// Takes a namespacedName string as a paremeter and
// deletes the ingress object from the cacheStores map
func (d *Driver) DeleteIngress(n types.NamespacedName) error {
	ingress := &netv1.Ingress{}
	// set NamespacedName on the ingress object
	ingress.SetNamespace(n.Namespace)
	ingress.SetName(n.Name)
	return d.cacheStores.Delete(ingress)
}
