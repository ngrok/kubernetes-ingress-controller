/*
Copyright 2017 The Kubernetes Authors.

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

package store

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	ingressv1alpha1 "github.com/ngrok/ngrok-ingress-controller/api/v1alpha1"
	"gopkg.in/yaml.v2"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructuredscheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

// ErrNotFound error is returned when a lookup results in no resource.
// This type is meant to be used for error handling using `errors.As()`.
type ErrNotFound struct {
	message string
}

// TODO: Make a more generically useable errors pkg for the project and move this there
// Also add a helper IsNotFound check
func (e ErrNotFound) Error() string {
	if e.message == "" {
		return "not found"
	}
	return e.message
}

// Storer is the interface that wraps the required methods to gather information
// about ingresses, services, secrets and ingress annotations.
type Storer interface {
	GetIngressClassV1(name string) (*netv1.IngressClass, error)

	ListIngressClassesV1() []*netv1.IngressClass
	ListIngressesV1() []*netv1.Ingress
	ListDomainsV1() []*ingressv1alpha1.Domain
	ListTunnelsV1() []*ingressv1alpha1.Tunnel
}

// Store implements Storer and can be used to list Ingress, Services
// and other resources from k8s APIserver. The backing stores should
// be synced and updated by the caller.
// It is ingressClass filter aware.
type Store struct {
	stores       CacheStores
	ingressClass string
}

var _ Storer = Store{}

// CacheStores stores cache.Store for all Kinds of k8s objects that
// the Ingress Controller reads.
type CacheStores struct {
	// Core Kubernetes Stores
	IngressV1      cache.Store
	IngressClassV1 cache.Store

	// Ngrok Stores
	DomainV1 cache.Store
	TunnelV1 cache.Store

	l *sync.RWMutex
}

// NewCacheStores is a convenience function for CacheStores to initialize all attributes with new cache stores.
func NewCacheStores() CacheStores {
	return CacheStores{
		IngressV1:      cache.NewStore(cache.MetaNamespaceKeyFunc),
		IngressClassV1: cache.NewStore(cache.MetaNamespaceKeyFunc),
		DomainV1:       cache.NewStore(cache.MetaNamespaceKeyFunc),
		TunnelV1:       cache.NewStore(cache.MetaNamespaceKeyFunc),
		l:              &sync.RWMutex{},
	}
}

// NewCacheStoresFromObjYAML provides a new CacheStores object given any number of byte arrays containing
// YAML Kubernetes objects. An error is returned if any provided YAML was not a valid Kubernetes object.
func NewCacheStoresFromObjYAML(objs ...[]byte) (c CacheStores, err error) {
	kobjs := make([]runtime.Object, 0, len(objs))
	sr := serializer.NewYAMLSerializer(
		yamlserializer.DefaultMetaFactory,
		unstructuredscheme.NewUnstructuredCreator(),
		unstructuredscheme.NewUnstructuredObjectTyper(),
	)
	for _, yaml := range objs {
		kobj, _, decodeErr := sr.Decode(yaml, nil, nil)
		if err = decodeErr; err != nil {
			return
		}
		kobjs = append(kobjs, kobj)
	}
	return NewCacheStoresFromObjs(kobjs...)
}

// NewCacheStoresFromObjs provides a new CacheStores object given any number of Kubernetes
// objects that should be pre-populated. This function will sort objects into the appropriate
// sub-storage (e.g. IngressV1, TCPIngress, e.t.c.) but will produce an error if any of the
// input objects are erroneous or otherwise unusable as Kubernetes objects.
func NewCacheStoresFromObjs(objs ...runtime.Object) (CacheStores, error) {
	c := NewCacheStores()
	for _, obj := range objs {
		typedObj, err := mkObjFromGVK(obj.GetObjectKind().GroupVersionKind())
		if err != nil {
			return c, err
		}

		if err := convUnstructuredObj(obj, typedObj); err != nil {
			return c, err
		}

		if err := c.Add(typedObj); err != nil {
			return c, err
		}
	}
	return c, nil
}

// Get checks whether or not there's already some version of the provided object present in the cache.
// The CacheStore must be initialized (see NewCacheStores()) or this will panic.
func (c CacheStores) Get(obj runtime.Object) (item interface{}, exists bool, err error) {
	c.l.RLock()
	defer c.l.RUnlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kubernetes Core API Support
	// ----------------------------------------------------------------------------
	case *netv1.Ingress:
		return c.IngressV1.Get(obj)
	case *netv1.IngressClass:
		return c.IngressClassV1.Get(obj)
		// ----------------------------------------------------------------------------
	// Ngrok API Support
	// ----------------------------------------------------------------------------
	case *ingressv1alpha1.Domain:
		return c.DomainV1.Get(obj)
	case *ingressv1alpha1.Tunnel:
		return c.TunnelV1.Get(obj)
	}

	return nil, false, fmt.Errorf("unsupported object type: %T", obj)
}

// Add stores a provided runtime.Object into the CacheStore if it's of a supported type.
// The CacheStore must be initialized (see NewCacheStores()) or this will panic.
func (c CacheStores) Add(obj runtime.Object) error {
	c.l.Lock()
	defer c.l.Unlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kubernetes Core API Support
	// ----------------------------------------------------------------------------
	case *netv1.Ingress:
		return c.IngressV1.Add(obj)
	case *netv1.IngressClass:
		return c.IngressClassV1.Add(obj)
		// ----------------------------------------------------------------------------
	// Ngrok API Support
	// ----------------------------------------------------------------------------
	case *ingressv1alpha1.Domain:
		return c.DomainV1.Add(obj)
	case *ingressv1alpha1.Tunnel:
		return c.TunnelV1.Add(obj)
	default:
		return fmt.Errorf("unsupported object type: %T", obj)
	}
}

// Delete removes a provided runtime.Object from the CacheStore if it's of a supported type.
// The CacheStore must be initialized (see NewCacheStores()) or this will panic.
func (c CacheStores) Delete(obj runtime.Object) error {
	c.l.Lock()
	defer c.l.Unlock()

	switch obj := obj.(type) {
	// ----------------------------------------------------------------------------
	// Kubernetes Core API Support
	// ----------------------------------------------------------------------------
	case *netv1.Ingress:
		return c.IngressV1.Delete(obj)
	case *netv1.IngressClass:
		return c.IngressClassV1.Delete(obj)
		// ----------------------------------------------------------------------------
	// Ngrok API Support
	// ----------------------------------------------------------------------------
	case *ingressv1alpha1.Domain:
		return c.DomainV1.Delete(obj)
	case *ingressv1alpha1.Tunnel:
		return c.TunnelV1.Delete(obj)
	default:
		return fmt.Errorf("unsupported object type: %T", obj)
	}
}

// GetIngressClassV1 returns the 'name' IngressClass resource.
func (s Store) GetIngressClassV1(name string) (*netv1.IngressClass, error) {
	p, exists, err := s.stores.IngressClassV1.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound{fmt.Sprintf("IngressClass %v not found", name)}
	}
	return p.(*netv1.IngressClass), nil
}

// ListIngressClassesV1 returns the list of Ingresses in the Ingress v1 store.
func (s Store) ListIngressClassesV1() []*netv1.IngressClass {
	// filter ingress rules
	var classes []*netv1.IngressClass
	for _, item := range s.stores.IngressClassV1.List() {
		class, ok := item.(*netv1.IngressClass)
		if !ok {
			// s.logger.Warnf("listIngressClassesV1: dropping object of unexpected type: %#v", item)
			continue
		}
		if class.Spec.Controller != "k8s.ngrok.com/ingress-controller" {
			continue
		}
		classes = append(classes, class)
	}

	sort.SliceStable(classes, func(i, j int) bool {
		return strings.Compare(classes[i].Name, classes[j].Name) < 0
	})

	return classes
}

// ListIngressesV1 returns the list of Ingresses in the Ingress v1 store.
func (s Store) ListIngressesV1() []*netv1.Ingress {
	// filter ingress rules
	var ingresses []*netv1.Ingress
	for _, item := range s.stores.IngressV1.List() {
		ing, ok := item.(*netv1.Ingress)
		if !ok {
			// s.logger.Warnf("listIngressesV1: dropping object of unexpected type: %#v", item)
			continue
		}
		// if ing.ObjectMeta.GetAnnotations()[annotations.IngressClassKey] != "" {
		// 	if !s.isValidIngressClass(&ing.ObjectMeta, annotations.IngressClassKey, s.ingressV1ClassMatching) {
		// 		continue
		// 	}
		// } else if ing.Spec.IngressClassName != nil {
		// 	if !s.isValidIngressV1Class(ing, s.ingressV1ClassMatching) {
		// 		continue
		// 	}
		// } else {
		// 	class, err := s.GetIngressClassV1(s.ingressClass)
		// 	if err != nil {
		// 		s.logger.Debugf("IngressClass %s not found", s.ingressClass)
		// 		continue
		// 	}
		// 	if !ctrlutils.IsDefaultIngressClass(class) {
		// 		continue
		// 	}
		// }
		ingresses = append(ingresses, ing)
	}

	sort.SliceStable(ingresses, func(i, j int) bool {
		return strings.Compare(fmt.Sprintf("%s/%s", ingresses[i].Namespace, ingresses[i].Name),
			fmt.Sprintf("%s/%s", ingresses[j].Namespace, ingresses[j].Name)) < 0
	})

	return ingresses
}

// ListDomainsV1 returns the list of Domains in the Domain v1 store.
func (s Store) ListDomainsV1() []*ingressv1alpha1.Domain {
	// filter ingress rules
	var domains []*ingressv1alpha1.Domain
	for _, item := range s.stores.DomainV1.List() {
		domain, ok := item.(*ingressv1alpha1.Domain)
		if !ok {
			// s.logger.Warnf("listDomainsV1: dropping object of unexpected type: %#v", item)
			continue
		}
		domains = append(domains, domain)
	}

	sort.SliceStable(domains, func(i, j int) bool {
		return strings.Compare(fmt.Sprintf("%s/%s", domains[i].Namespace, domains[i].Name),
			fmt.Sprintf("%s/%s", domains[j].Namespace, domains[j].Name)) < 0
	})

	return domains
}

// ListTunnelsV1 returns the list of Tunnels in the Tunnel v1 store.
func (s Store) ListTunnelsV1() []*ingressv1alpha1.Tunnel {
	var tunnels []*ingressv1alpha1.Tunnel
	for _, item := range s.stores.TunnelV1.List() {
		tunnel, ok := item.(*ingressv1alpha1.Tunnel)
		if !ok {
			// s.logger.Warnf("listTunnelsV1: dropping object of unexpected type: %#v", item)
			continue
		}
		tunnels = append(tunnels, tunnel)
	}

	sort.SliceStable(tunnels, func(i, j int) bool {
		return strings.Compare(fmt.Sprintf("%s/%s", tunnels[i].Namespace, tunnels[i].Name),
			fmt.Sprintf("%s/%s", tunnels[j].Namespace, tunnels[j].Name)) < 0
	})

	return tunnels
}

// convUnstructuredObj is a convenience function to quickly convert any runtime.Object where the underlying type
// is an *unstructured.Unstructured (client-go's dynamic client type) and convert that object to a runtime.Object
// which is backed by the API type it represents. You can use the GVK of the runtime.Object to determine what type
// you want to convert to. This function is meant so that storer implementations can optionally work with YAML files
// for caller convenience when initializing new CacheStores objects.
//
// TODO: upon some searching I didn't find an analog to this over in client-go (https://github.com/kubernetes/client-go)
// however I could have just missed it. We should switch if we find something better, OR we should contribute
// this functionality upstream.
func convUnstructuredObj(from, to runtime.Object) error {
	b, err := yaml.Marshal(from)
	if err != nil {
		return fmt.Errorf("failed to convert object %s to yaml: %w", from.GetObjectKind().GroupVersionKind(), err)
	}
	return yaml.Unmarshal(b, to)
}

// mkObjFromGVK is a factory function that returns a concrete implementation runtime.Object
// for the given GVK. Callers can then use `convert()` to convert an unstructured
// runtime.Object into a concrete one.
func mkObjFromGVK(gvk schema.GroupVersionKind) (runtime.Object, error) {
	switch gvk {
	case netv1.SchemeGroupVersion.WithKind("Ingress"):
		return &netv1.Ingress{}, nil
	case netv1.SchemeGroupVersion.WithKind("IngressClass"):
		return &netv1.IngressClass{}, nil
	case ingressv1alpha1.GroupVersion.WithKind("Domain"):
		return &ingressv1alpha1.Domain{}, nil
	case ingressv1alpha1.GroupVersion.WithKind("Tunnel"):
		return &ingressv1alpha1.Tunnel{}, nil
	default:
		return nil, fmt.Errorf("unknown GVK: %v", gvk)
	}
}
