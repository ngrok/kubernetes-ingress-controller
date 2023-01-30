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
	"reflect"
	"sort"
	"strings"
	"sync"

	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	"github.com/ngrok/kubernetes-ingress-controller/internal/errors"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/go-logr/logr"
)

func keyFunc(obj interface{}) (string, error) {
	v := reflect.Indirect(reflect.ValueOf(obj))
	name := v.FieldByName("Name")
	namespace := v.FieldByName("Namespace")
	return namespace.String() + "/" + name.String(), nil
}

// Storer is the interface that wraps the required methods to gather information
// about ingresses, services, secrets and ingress annotations.
type Storer interface {
	GetIngressClassV1(name string) (*netv1.IngressClass, error)

	ListIngressClassesV1() []*netv1.IngressClass
	ListNgrokIngressClassesV1() []*netv1.IngressClass
	ListIngressesV1() []*netv1.Ingress
	ListNgrokIngressesV1() []*netv1.Ingress
	ListDomainsV1() []*ingressv1alpha1.Domain
	ListTunnelsV1() []*ingressv1alpha1.Tunnel

	GetIngressV1(name, namespace string) (*netv1.Ingress, error)
	GetNgrokIngressV1(name, namespace string) (*netv1.Ingress, error)
}

// Store implements Storer and can be used to list Ingress, Services
// and other resources from k8s APIserver. The backing stores should
// be synced and updated by the caller.
// It is ingressClass filter aware.
type Store struct {
	stores         CacheStores
	controllerName string
	log            logr.Logger
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

	log logr.Logger
	l   *sync.RWMutex
}

// NewCacheStores is a convenience function for CacheStores to initialize all attributes with new cache stores.
func NewCacheStores(logger logr.Logger) CacheStores {
	return CacheStores{
		IngressV1:      cache.NewStore(keyFunc),
		IngressClassV1: cache.NewStore(keyFunc),
		DomainV1:       cache.NewStore(keyFunc),
		TunnelV1:       cache.NewStore(keyFunc),
		l:              &sync.RWMutex{},
		log:            logger,
	}
}

// New creates a new object store to be used in the ingress controller.
func New(cs CacheStores, controllerName string, logger logr.Logger) Storer {
	return Store{
		stores:         cs,
		controllerName: controllerName,
		log:            logger,
	}
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
		return nil, errors.NewErrorNotFound(fmt.Sprintf("IngressClass %v not found", name))
	}
	return p.(*netv1.IngressClass), nil
}

// GetIngressV1 returns the 'name' Ingress resource.
func (s Store) GetIngressV1(name, namespcae string) (*netv1.Ingress, error) {
	p, exists, err := s.stores.IngressV1.GetByKey(fmt.Sprintf("%v/%v", namespcae, name))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewErrorNotFound(fmt.Sprintf("Ingress %v not found", name))
	}
	return p.(*netv1.Ingress), nil
}

func (s Store) GetNgrokIngressV1(name, namespace string) (*netv1.Ingress, error) {
	ing, err := s.GetIngressV1(name, namespace)
	if err != nil {
		return nil, err
	}
	if !s.shouldHandleIngress(ing) {
		ngrokClasses := s.ListNgrokClassNames()

		return nil, errors.NewErrDifferentIngressClass(ngrokClasses, *ing.Spec.IngressClassName)
	}

	return ing, nil
}

// ListIngressClassesV1 returns the list of Ingresses in the Ingress v1 store.
func (s Store) ListIngressClassesV1() []*netv1.IngressClass {
	// filter ingress rules
	var classes []*netv1.IngressClass
	for _, item := range s.stores.IngressClassV1.List() {
		class, ok := item.(*netv1.IngressClass)
		if !ok {
			s.log.Info("listIngressClassesV1: dropping object of unexpected type: %#v", item)
			continue
		}
		classes = append(classes, class)
	}

	sort.SliceStable(classes, func(i, j int) bool {
		return strings.Compare(classes[i].Name, classes[j].Name) < 0
	})

	return classes
}

// ListNgrokIngressClassesV1 returns the list of Ingresses in the Ingress v1 store filtered
// by ones that match the controllerName
func (s Store) ListNgrokIngressClassesV1() []*netv1.IngressClass {
	filteredClasses := []*netv1.IngressClass{}
	classes := s.ListIngressClassesV1()
	for _, class := range classes {
		if class.Spec.Controller == s.controllerName {
			filteredClasses = append(filteredClasses, class)
		}
	}

	return filteredClasses
}

// ListNgrokClassNames returns a string slice of the names of each matching ingress class
func (s Store) ListNgrokClassNames() []string {
	classes := s.ListNgrokIngressClassesV1()
	names := []string{}
	for _, class := range classes {
		names = append(names, class.Name)
	}
	return names
}

// ListIngressesV1 returns the list of Ingresses in the Ingress v1 store.
func (s Store) ListIngressesV1() []*netv1.Ingress {
	// filter ingress rules
	var ingresses []*netv1.Ingress

	for _, item := range s.stores.IngressV1.List() {
		ing, ok := item.(*netv1.Ingress)
		if !ok {
			e := fmt.Sprintf("listIngressesV1: dropping object of unexpected type: %#v", item)
			s.log.Error(fmt.Errorf(e), e)
			continue
		}
		ingresses = append(ingresses, ing)
	}

	sort.SliceStable(ingresses, func(i, j int) bool {
		return strings.Compare(fmt.Sprintf("%s/%s", ingresses[i].Namespace, ingresses[i].Name),
			fmt.Sprintf("%s/%s", ingresses[j].Namespace, ingresses[j].Name)) < 0
	})

	return ingresses
}

func (s Store) ListNgrokIngressesV1() []*netv1.Ingress {
	ings := s.ListIngressesV1()

	var ingresses []*netv1.Ingress
	for _, ing := range ings {
		if s.shouldHandleIngress(ing) {
			ingresses = append(ingresses, ing)
		}
	}
	return ingresses
}

func (s Store) shouldHandleIngress(ing *netv1.Ingress) bool {
	return s.shouldHandleIngressCheckClass(ing) && s.shouldHandleIngressIsValid(ing)
}

// shouldHandleIngressCheckClass checks if the ingress should be handled by the controller based on the ingress class
func (s Store) shouldHandleIngressCheckClass(ing *netv1.Ingress) bool {
	ngrokClasses := s.ListNgrokIngressClassesV1()
	if ing.Spec.IngressClassName != nil {
		for _, class := range ngrokClasses {
			if *ing.Spec.IngressClassName == class.Name {
				return true
			}
		}
	} else {
		for _, class := range ngrokClasses {
			if class.ObjectMeta.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
				return true
			}
		}
	}
	return false
}

// shouldHandleIngressIsValid checks if the ingress should be handled by the controller based on the ingress spec
func (s Store) shouldHandleIngressIsValid(ing *netv1.Ingress) bool {
	var err error
	if len(ing.Spec.Rules) > 1 {
		err = fmt.Errorf("A maximum of one rule is required to be set")
	}
	if len(ing.Spec.Rules) == 0 {
		err = fmt.Errorf("At least one rule is required to be set")
	}
	if ing.Spec.Rules[0].Host == "" {
		err = fmt.Errorf("A host is required to be set")
	}
	for _, path := range ing.Spec.Rules[0].HTTP.Paths {
		if path.Backend.Resource != nil {
			err = fmt.Errorf("Resource backends are not supported")
		}
	}
	if err != nil {
		s.log.Error(err, "Ingress is invalid")
		return false
	}
	return true
}

// ListDomainsV1 returns the list of Domains in the Domain v1 store.
func (s Store) ListDomainsV1() []*ingressv1alpha1.Domain {
	// filter ingress rules
	var domains []*ingressv1alpha1.Domain
	for _, item := range s.stores.DomainV1.List() {
		domain, ok := item.(*ingressv1alpha1.Domain)
		if !ok {
			s.log.Info("listDomainsV1: dropping object of unexpected type: %#v", item)
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
			s.log.Info("listTunnelsV1: dropping object of unexpected type: %#v", item)
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
