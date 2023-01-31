package store

import (
	"context"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var _ handler.EventHandler = &EnqueueOwnersAfterSyncingHandler{}

type EnqueueOwnersAfterSyncingHandler struct {
	// TODO:(initial-store) I'm not actually sure if this is necessary. If all these calls do the .Sync
	// then we don't have to trigger a reconcile of the ingress object to get the data to be posted
	// This also isn't correct for some types like ingress classes. Ingresses aren't the owners
	// i don't think. I don't think you do it the other way around either?
	ownerHandler handler.EnqueueRequestForOwner
	driver       *Driver
	log          logr.Logger
	client       client.Client
}

func NewEnqueueOwnersAfterSyncingHandler(resourceName string, d *Driver, c client.Client) *EnqueueOwnersAfterSyncingHandler {
	return &EnqueueOwnersAfterSyncingHandler{
		ownerHandler: handler.EnqueueRequestForOwner{
			IsController: false, // TODO:(initial-store) Figure out owner vs controller and see if this works
			OwnerType:    &netv1.Ingress{},
		},
		driver: d,
		log:    d.log.WithValues("EnqueueOwnersAfterSyncingHandlerFor", resourceName),
		client: c,
	}
}

func (e *EnqueueOwnersAfterSyncingHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Update(evt.Object)
	if err != nil {
		e.log.Error(err, "error updating object", "object", evt.Object)
		return
	}
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Create(evt, q)
}

func (e *EnqueueOwnersAfterSyncingHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Update(evt.ObjectNew)
	if err != nil {
		e.log.Error(err, "error updating object", "object", evt.ObjectNew)
		return
	}
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Update(evt, q)
}

func (e *EnqueueOwnersAfterSyncingHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Delete(evt.Object)
	if err != nil {
		e.log.Error(err, "error deleting object", "object", evt.Object)
		return
	}
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Delete(evt, q)
}

func (e *EnqueueOwnersAfterSyncingHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	err := e.driver.Update(evt.Object)
	if err != nil {
		e.log.Error(err, "error updating object", "object", evt.Object)
		return
	}
	// Sync then call OwnersHandler
	e.driver.Sync(context.Background(), e.client)
	e.ownerHandler.Generic(evt, q)
}
