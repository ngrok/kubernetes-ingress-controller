package store

import (
	"sync"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type Driver struct {
	Store       Storer
	cacheStores CacheStores
	log         logr.Logger
	l           *sync.RWMutex
}

func NewDriver(logger logr.Logger) *Driver {
	cacheStores := NewCacheStores(logger)
	s := New(cacheStores, "ngrok", false, true, logger)
	return &Driver{
		Store:       s,
		cacheStores: cacheStores,
		log:         logger,
		l:           &sync.RWMutex{},
	}
}

func (d *Driver) Delete(obj runtime.Object) error {
	return d.cacheStores.Delete(obj)
}

// TODO: Probably remove the mutex here and also maybe rename to Update
func (d *Driver) Add(obj runtime.Object) error {
	d.l.Lock()
	defer d.l.Unlock()

	return d.cacheStores.Add(obj.DeepCopyObject())
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
