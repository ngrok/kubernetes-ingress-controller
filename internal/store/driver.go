package store

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The name of the ingress controller which is uses to match on ingress classes
const controllerName = "k8s.ngrok.com/ingress-controller" // TODO:(initial-store) Let the user configure this
const clusterDomain = "svc.cluster.local"                 // TODO: We can technically figure this out by looking at things like our resolv.conf or we can just take this as a helm option

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

// func (d *Driver) ListIngressesV1ForDomains(domains []string) ([]*netv1.Ingress, error) {
// 	matchingIngs := make([]*netv1.Ingress, 0)
// 	ingresses := d.Store.ListNgrokIngressesV1()
// 	for _, ingress := range ingresses {
// 		for _, rule := range ingress.Spec.Rules {
// 			for _, domain := range domains {
// 				// If the rule host matches the domain, or if there is no host for the rule
// 				// then it applies so we technically match
// 				if rule.Host == domain || rule.Host == "" {
// 					matchingIngs = append(matchingIngs, ingress)
// 					break
// 				}
// 			}
// 		}
// 	}
// }

type empty struct{}

var _ handler.EventHandler = &EnqueueOwnersAfterSyncing{}

// func EnqueueOwnersAfterSyncing

type EnqueueOwnersAfterSyncing struct {
	ownerHandler handler.EnqueueRequestForOwner
	driver       *Driver
	client       client.Client
}

func NewEnqueueOwnersAfterSyncing(d *Driver, c client.Client) *EnqueueOwnersAfterSyncing {
	return &EnqueueOwnersAfterSyncing{
		ownerHandler: handler.EnqueueRequestForOwner{
			IsController: false, // TODO: Figure out owner vs controller and see if this works
			OwnerType:    &netv1.Ingress{},
		},
		driver: d,
		client: c,
	}
}

func (e *EnqueueOwnersAfterSyncing) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Update(evt.Object)
	if err != nil {
		e.driver.log.Error(err, "error updating object", "object", evt.Object)
		return
	}
	e.driver.log.Info("Enqueue called for Create and passed")
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Create(evt, q)
}

func (e *EnqueueOwnersAfterSyncing) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Update(evt.ObjectNew)
	if err != nil {
		e.driver.log.Error(err, "error updating object", "object", evt.ObjectNew)
		return
	}
	e.driver.log.Info("Enqueue called for Update and passed")
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Update(evt, q)
}

func (e *EnqueueOwnersAfterSyncing) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Delete(evt.Object)
	if err != nil {
		e.driver.log.Error(err, "error deleting object", "object", evt.Object)
		return
	}
	e.driver.log.Info("Enqueue called for Delete and passed")
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Delete(evt, q)
}

func (e *EnqueueOwnersAfterSyncing) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Update(evt.Object)
	if err != nil {
		e.driver.log.Error(err, "error updating object", "object", evt.Object)
		return
	}
	e.driver.log.Info("Enqueue called for Generic and passed")
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Generic(evt, q)
}

func (d *Driver) Sync(ctx context.Context, c client.Client) error {
	d.log.Info("syncing driver state!!")
	// - calculates new domains
	// 	- makes a call to get the current ones
	// 	- diffs them, and reconciles the results
	// 	- update status associated ingress objects with load balancer HostName (via k8s api
	// - calculates new edges
	// 	- makes a call to get the current ones
	// 	- diffs them, and reconciles the results
	// - calculates new tunnels
	// 	- makes a call to get the current ones
	// 	- diffs them, and reconciles the results
	// - TODO: Probably set SetOwnerReference for each instead of how we do SetControllerReference

	return nil
}

func ingressToTunnels(ingress *netv1.Ingress) []ingressv1alpha1.Tunnel {
	tunnels := make([]ingressv1alpha1.Tunnel, 0)

	if ingress == nil || len(ingress.Spec.Rules) == 0 {
		return tunnels
	}

	// Tunnels should be unique on a service and port basis so if they are referenced more than once, we
	// only create one tunnel per service and port.
	tunnelMap := make(map[string]ingressv1alpha1.Tunnel)
	for _, rule := range ingress.Spec.Rules {
		if rule.Host == "" {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			serviceName := path.Backend.Service.Name
			servicePort := path.Backend.Service.Port.Number
			tunnelAddr := fmt.Sprintf("%s.%s.%s:%d", serviceName, ingress.Namespace, clusterDomain, servicePort)
			tunnelName := fmt.Sprintf("%s-%d", serviceName, servicePort)

			tunnelMap[tunnelName] = ingressv1alpha1.Tunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tunnelName,
					Namespace: ingress.Namespace,
				},
				Spec: ingressv1alpha1.TunnelSpec{
					ForwardsTo: tunnelAddr,
					Labels:     backendToLabelMap(path.Backend, ingress.Namespace),
				},
			}
		}
	}

	for _, tunnel := range tunnelMap {
		tunnels = append(tunnels, tunnel)
	}

	return tunnels
}

// Generates a labels map for matching ngrok Routes to Agent Tunnels
func backendToLabelMap(backend netv1.IngressBackend, namespace string) map[string]string {
	return map[string]string{
		"k8s.ngrok.com/namespace": namespace,
		"k8s.ngrok.com/service":   backend.Service.Name,
		"k8s.ngrok.com/port":      strconv.Itoa(int(backend.Service.Port.Number)),
	}
}
