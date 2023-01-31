package controllers

import (
	"context"

	"github.com/go-logr/logr"
	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	internalerrors "github.com/ngrok/kubernetes-ingress-controller/internal/errors"
	"github.com/ngrok/kubernetes-ingress-controller/internal/store"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// This implements the Reconciler for the controller-runtime
// https://pkg.go.dev/sigs.k8s.io/controller-runtime#section-readme
type IngressReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Driver    *store.Driver
}

// Create a new controller using our reconciler and set it up with the manager
func (irec *IngressReconciler) SetupWithManager(mgr ctrl.Manager, d *store.Driver) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1.Ingress{}).
		// TODO:(initial-store): Watch ingress classes and create a basic function to find all ings for thats class
		Watches(
			&source.Kind{Type: &ingressv1alpha1.Domain{}},
			store.NewEnqueueOwnersAfterSyncing(d, mgr.GetClient()),
		).
		Watches(
			&source.Kind{Type: &ingressv1alpha1.HTTPSEdge{}},
			store.NewEnqueueOwnersAfterSyncing(d, mgr.GetClient()),
		).
		Watches(
			&source.Kind{Type: &ingressv1alpha1.Tunnel{}},
			store.NewEnqueueOwnersAfterSyncing(d, mgr.GetClient()),
		).
		Complete(irec)
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses/status,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingressclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// This reconcile function is called by the controller-runtime manager.
// It is invoked whenever there is an event that occurs for a resource
// being watched (in our case, ingress objects). If you tail the controller
// logs and delete, update, edit ingress objects, you see the events come in.
func (irec *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := irec.Log.WithValues("ingress", req.NamespacedName)
	ctx = ctrl.LoggerInto(ctx, log)
	ingress := &netv1.Ingress{}
	err := irec.Client.Get(ctx, req.NamespacedName, ingress)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// If its fully gone, delete it from the store
			return ctrl.Result{}, irec.Driver.DeleteIngress(req.NamespacedName)
		}
		return ctrl.Result{}, err // Otherwise, its a real error
	}

	// Ensure the ingress object is up to date in the store
	err = irec.Driver.Update(ingress)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Even though we already have the ingress object, leverage the store to ensure this works off the same data as everything else
	ingress, err = irec.Driver.Store.GetNgrokIngressV1(ingress.Name, ingress.Namespace)
	if internalerrors.IsErrDifferentIngressClass(err) {
		log.Info("Ingress is not of type ngrok so skipping it")
		return ctrl.Result{}, nil
	}
	if internalerrors.IsErrInvalidIngressSpec(err) {
		log.Info("Ingress is not valid so skipping it")
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "Failed to get ingress from store")
		return ctrl.Result{}, err
	}

	if ingress.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so register and sync finalizer
		if err := registerAndSyncFinalizer(ctx, irec.Client, ingress); err != nil {
			log.Error(err, "Failed to register finalizer")
			return ctrl.Result{}, err
		}
	} else {
		// The object is being deleted
		if hasFinalizer(ingress) {
			log.Info("Deleting ingress")

			if err = irec.DeleteDependents(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}

			if err := removeAndSyncFinalizer(ctx, irec.Client, ingress); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}

			// Remove the ingress object from the store
			return ctrl.Result{}, irec.Driver.DeleteIngress(req.NamespacedName)
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	return irec.reconcileAll(ctx, ingress)
}

func (irec *IngressReconciler) DeleteDependents(ctx context.Context, ingress *netv1.Ingress) error {
	// TODO:(initial-store) Currently this controller "owns" the HTTPSEdge and Tunnel objects so deleting an ingress
	// will delete the HTTPSEdge and Tunnel objects. Once multiple ingress objects combine to form 1 edge
	// this logic will need to be smarter
	return nil
}

func (irec *IngressReconciler) reconcileAll(ctx context.Context, ingress *netv1.Ingress) (reconcile.Result, error) {
	log := irec.Log
	// First Update the store
	err := irec.Driver.Update(ingress)
	if err != nil {
		log.Error(err, "Failed to add ingress to store")
		return ctrl.Result{}, err
	}

	err = irec.Driver.Sync(ctx, irec.Client)
	if err != nil {
		log.Error(err, "Failed to sync ingress to store")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
