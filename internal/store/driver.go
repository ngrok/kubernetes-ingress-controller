package store

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	"github.com/ngrok/kubernetes-ingress-controller/internal/annotations"
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
	scheme      *runtime.Scheme
}

// NewDriver creates a new driver with a basic logger and cache store setup
func NewDriver(logger logr.Logger, scheme *runtime.Scheme) *Driver {
	cacheStores := NewCacheStores(logger)
	s := New(cacheStores, controllerName, logger)
	return &Driver{
		Store:       s,
		cacheStores: cacheStores,
		log:         logger,
		scheme:      scheme,
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
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Generic(evt, q)
}

// TODO:(initial-store): This thing needs to be dried up somehow
func (d *Driver) Sync(ctx context.Context, c client.Client) error {
	d.log.Info("syncing driver state!!")
	desiredDomains := d.calculateDomains()
	desiredEdges := d.calculateHTTPSEdges()
	desiredTunnels := d.calculateTunnels()

	currDomains := &ingressv1alpha1.DomainList{}
	currEdges := &ingressv1alpha1.HTTPSEdgeList{}
	currTunnels := &ingressv1alpha1.TunnelList{}

	if err := c.List(ctx, currDomains); err != nil {
		d.log.Error(err, "error listing domains")
		return err
	}
	if err := c.List(ctx, currEdges); err != nil {
		d.log.Error(err, "error listing edges")
		return err
	}
	if err := c.List(ctx, currTunnels); err != nil {
		d.log.Error(err, "error listing tunnels")
		return err
	}

	for _, desiredDomain := range desiredDomains {
		found := false
		for _, currDomain := range currDomains.Items {
			if desiredDomain.Name == currDomain.Name {
				// It matches so lets update it if anything is different
				if !reflect.DeepEqual(desiredDomain.Spec, currDomain.Spec) {
					currDomain.Spec = desiredDomain.Spec
					if err := c.Update(ctx, &currDomain); err != nil {
						d.log.Error(err, "error updating domain", "domain", desiredDomain)
						return err
					}
				}
				found = true
				break
			}
		}
		if !found {
			if err := c.Create(ctx, &desiredDomain); err != nil {
				d.log.Error(err, "error creating domain", "domain", desiredDomain)
				return err
			}
			break
		}
	}

	// TODO:(initial-store) Determine if we should delete domains if they are no longer used
	// for _, existingDomain := range currDomains.Items {
	// 	found := false
	// 	for _, desiredDomain := range desiredDomains {
	// 		if desiredDomain.Name == existingDomain.Name {
	// 			found = true
	// 			break
	// 		}
	// 	}
	// 	if !found {
	// 		if err := c.Delete(ctx, &existingDomain); err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	for _, desiredEdge := range desiredEdges {
		found := false
		for _, currEdge := range currEdges.Items {
			if desiredEdge.Name == currEdge.Name {
				// It matches so lets update it if anything is different
				if !reflect.DeepEqual(desiredEdge.Spec, currEdge.Spec) {
					currEdge.Spec = desiredEdge.Spec
					if err := c.Update(ctx, &currEdge); err != nil {
						return err
					}
				}
				found = true
				break
			}
		}
		if !found {
			if err := c.Create(ctx, &desiredEdge); err != nil {
				return err
			}
			break
		}
	}

	for _, existingEdge := range currEdges.Items {
		found := false
		for _, desiredEdge := range desiredEdges {
			if desiredEdge.Name == existingEdge.Name {
				found = true
				break
			}
		}
		if !found {
			if err := c.Delete(ctx, &existingEdge); client.IgnoreNotFound(err) != nil {
				d.log.Error(err, "error deleting edge", "edge", existingEdge)
				return err
			}
		}
	}

	for _, desiredTunnel := range desiredTunnels {
		found := false
		for _, currTunnel := range currTunnels.Items {
			if desiredTunnel.Name == currTunnel.Name {
				// It matches so lets update it if anything is different
				if !reflect.DeepEqual(desiredTunnel.Spec, currTunnel.Spec) {
					currTunnel.Spec = desiredTunnel.Spec
					if err := c.Update(ctx, &currTunnel); err != nil {
						d.log.Error(err, "error updating tunnel", "tunnel", desiredTunnel)
						return err
					}
				}
				found = true
				break
			}
		}
		if !found {
			if err := c.Create(ctx, &desiredTunnel); err != nil {
				d.log.Error(err, "error creating tunnel", "tunnel", desiredTunnel)
				return err
			}
			break
		}
	}

	for _, existingTunnel := range currTunnels.Items {
		found := false
		for _, desiredTunnel := range desiredTunnels {
			if desiredTunnel.Name == existingTunnel.Name {
				found = true
				break
			}
		}
		if !found {
			if err := c.Delete(ctx, &existingTunnel); client.IgnoreNotFound(err) != nil {
				d.log.Error(err, "error deleting tunnel", "tunnel", existingTunnel)
				return err
			}
		}
	}

	// TODO:(initial-store) - update the ingress objects status with the load balancer hostname
	return nil
}

// TODO:(initial-store) - set the SetOwnerReference
func (d *Driver) calculateDomains() []ingressv1alpha1.Domain {
	// make a map of string to domains
	domainMap := make(map[string]ingressv1alpha1.Domain)
	ingresses := d.Store.ListNgrokIngressesV1()
	for _, ingress := range ingresses {
		for _, rule := range ingress.Spec.Rules {
			if rule.Host == "" {
				continue
			}
			domainMap[rule.Host] = ingressv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.Replace(rule.Host, ".", "-", -1),
					Namespace: ingress.Namespace,
				},
				Spec: ingressv1alpha1.DomainSpec{
					Domain: rule.Host,
				},
			}
		}
	}
	domains := make([]ingressv1alpha1.Domain, 0, len(domainMap))
	for _, domain := range domainMap {
		domains = append(domains, domain)
	}
	return domains
}

func (d *Driver) calculateHTTPSEdges() []ingressv1alpha1.HTTPSEdge {
	domains := d.calculateDomains()
	ingresses := d.Store.ListNgrokIngressesV1()
	edges := make([]ingressv1alpha1.HTTPSEdge, 0, len(domains))
	for _, domain := range domains {
		edge := ingressv1alpha1.HTTPSEdge{
			ObjectMeta: metav1.ObjectMeta{
				Name:      domain.Name,
				Namespace: domain.Namespace,
			},
			Spec: ingressv1alpha1.HTTPSEdgeSpec{
				Hostports: []string{domain.Spec.Domain + ":443"},
			},
		}
		var ngrokRoutes []ingressv1alpha1.HTTPSEdgeRouteSpec
		for _, ingress := range ingresses {
			for _, rule := range ingress.Spec.Rules {
				if rule.Host == domain.Spec.Domain {
					edge.GetAnnotations()
					err := controllerutil.SetOwnerReference(ingress, &edge, d.scheme)
					if err != nil {
						// TODO:(initial-store) handle this error
						panic(err)
					}
					var matchType string
					parsedRouteModules := annotations.NewAnnotationsExtractor().Extract(ingress)

					for _, httpIngressPath := range rule.HTTP.Paths {
						switch *httpIngressPath.PathType {
						case netv1.PathTypePrefix:
							matchType = "path_prefix"
						case netv1.PathTypeExact:
							matchType = "exact_path"
						case netv1.PathTypeImplementationSpecific:
							matchType = "path_prefix" // Path Prefix seems like a sane default for most cases
						default:
							d.log.Error(fmt.Errorf("unknown path type"), "unknown path type", "pathType", *httpIngressPath.PathType)
							return nil
						}

						route := ingressv1alpha1.HTTPSEdgeRouteSpec{
							Match:     httpIngressPath.Path,
							MatchType: matchType,
							Backend: ingressv1alpha1.TunnelGroupBackend{
								Labels: backendToLabelMap(httpIngressPath.Backend, ingress.Namespace),
							},
							Compression:   parsedRouteModules.Compression,
							IPRestriction: parsedRouteModules.IPRestriction,
							Headers:       parsedRouteModules.Headers,
						}

						ngrokRoutes = append(ngrokRoutes, route)
					}
				}
			}
		}
		// After all the ingresses, update the edge with the routes
		edge.Spec.Routes = ngrokRoutes
		edges = append(edges, edge)
	}

	return edges
}

// TODO:(initial-store) - set the SetOwnerReference
func (d *Driver) calculateTunnels() []ingressv1alpha1.Tunnel {
	// Tunnels should be unique on a service and port basis so if they are referenced more than once, we
	// only create one tunnel per service and port.
	tunnelMap := make(map[string]ingressv1alpha1.Tunnel)
	ingresses := d.Store.ListNgrokIngressesV1()
	for _, ingress := range ingresses {
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
	}

	tunnels := make([]ingressv1alpha1.Tunnel, 0, len(tunnelMap))
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
