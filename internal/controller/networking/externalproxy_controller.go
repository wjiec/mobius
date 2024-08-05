/*
Copyright 2024 Jayson Wang.

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

package networking

import (
	"context"
	"flag"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/wjiec/mobius/internal/expectations"
	"github.com/wjiec/mobius/internal/fieldindex"
	"github.com/wjiec/mobius/internal/patch"
	"github.com/wjiec/mobius/internal/sync/singleton"
	"github.com/wjiec/mobius/internal/sync/singleton/filter"
	"github.com/wjiec/mobius/pkg/utils"
)

func init() {
	flag.IntVar(&concurrentReconciles, "externalproxy-workers", concurrentReconciles, "Max concurrent workers for ExternalProxy controller")
}

var (
	concurrentReconciles = 3

	_ reconcile.Reconciler = (*Reconciler)(nil)
)

type Reconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	statusUpdater  StatusUpdater
	serviceSyncer  *ServiceSyncer
	endpointSyncer *EndpointsSyncer
	ingressSyncer  *IngressSyncer
}

// NewExternalProxyReconciler configures a ExternalProxy controller.
func NewExternalProxyReconciler(c client.Client, scheme *runtime.Scheme) *Reconciler {
	r := &Reconciler{
		Client:        c,
		Scheme:        scheme,
		statusUpdater: NewStatusUpdater(c),
	}

	r.serviceSyncer = singleton.New[*ExternalProxy, *corev1.Service](r.Scheme, r.newServiceSyncEventHandler())
	r.endpointSyncer = singleton.New[*ExternalProxy, *corev1.Endpoints](r.Scheme, r.newEndpointsSyncEventHandler())
	r.ingressSyncer = singleton.New[*ExternalProxy, *networkingv1.Ingress](r.Scheme, r.newIngressSyncEventHandler())

	return r
}

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.laboys.org,resources=externalproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.laboys.org,resources=externalproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.laboys.org,resources=externalproxies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = expectations.WithControllerKey(ctx, req.String())

	logger := log.FromContext(ctx)
	defer func(startTime time.Time) {
		logger.Info("Finished reconcile", "duration", time.Since(startTime))
	}(time.Now())

	var instance ExternalProxy
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			logger.V(4).Info("Deleted")
		}

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If services expectations have not satisfied yet, just skip this reconcile.
	if satisfied, unsatisfiedDuration := r.serviceSyncer.SatisfiedExpectations(req.String()); !satisfied {
		if unsatisfiedDuration >= expectations.ExpectationTimeout {
			logger.Info("Expectation unsatisfied overtime", "overTime", unsatisfiedDuration)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{RequeueAfter: expectations.ExpectationTimeout - unsatisfiedDuration}, nil
	}
	if err := r.serviceSyncer.Sync(ctx, &instance); err != nil {
		return ctrl.Result{}, err
	}

	// If endpoints expectations have not satisfied yet, just skip this reconcile.
	if satisfied, unsatisfiedDuration := r.endpointSyncer.SatisfiedExpectations(req.String()); !satisfied {
		if unsatisfiedDuration >= expectations.ExpectationTimeout {
			logger.Info("Expectation unsatisfied overtime", "overTime", unsatisfiedDuration)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{RequeueAfter: expectations.ExpectationTimeout - unsatisfiedDuration}, nil
	}
	if err := r.endpointSyncer.Sync(ctx, &instance); err != nil {
		return ctrl.Result{}, err
	}

	// If ingress expectations have not satisfied yet, just skip this reconcile.
	if satisfied, unsatisfiedDuration := r.ingressSyncer.SatisfiedExpectations(req.String()); !satisfied {
		if unsatisfiedDuration >= expectations.ExpectationTimeout {
			logger.Info("Expectation unsatisfied overtime", "overTime", unsatisfiedDuration)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{RequeueAfter: expectations.ExpectationTimeout - unsatisfiedDuration}, nil
	}
	if err := r.ingressSyncer.Sync(ctx, &instance); err != nil {
		return ctrl.Result{}, err
	}

	newStatus := &ExternalProxyStatus{
		Ready:              true,
		ServiceName:        getServiceName(&instance),
		ObservedGeneration: instance.Generation,
	}
	return ctrl.Result{}, r.statusUpdater.UpdateStatus(ctx, &instance, newStatus)
}

func (r *Reconciler) newServiceSyncEventHandler() *singleton.EventHandler[*ExternalProxy, *corev1.Service] {
	return &singleton.EventHandler[*ExternalProxy, *corev1.Service]{
		Writer:          r.Client,
		NewObject:       newService,
		ListObject:      r.listOwnedServices,
		ActivatedObject: filter.OverrideName[*ExternalProxy, *corev1.Service](getServiceName),
		ObjectRevision:  extractExternalProxyRevision[*corev1.Service],
		ObjectPatcher: func(controller *ExternalProxy, desiredObject, activatedObject *corev1.Service) (client.Patch, error) {
			object := activatedObject.DeepCopy()
			object.Spec.Type = desiredObject.Spec.Type
			object.Spec.Ports = desiredObject.Spec.Ports
			utils.Merge(&object.Labels, desiredObject.Labels)
			utils.Merge(&object.Annotations, desiredObject.Annotations)

			return patch.CreateTwoWayMergePatch(activatedObject, applyExternalProxyRevision(controller, object))
		},
	}
}

func (r *Reconciler) listOwnedServices(ctx context.Context, instance *ExternalProxy) ([]*corev1.Service, error) {
	var serviceList corev1.ServiceList
	if err := r.listOwnedResources(ctx, &serviceList, instance); err != nil {
		return nil, err
	}

	services := make([]*corev1.Service, 0, len(serviceList.Items))
	for i := range serviceList.Items {
		service := &serviceList.Items[i]
		if service.DeletionTimestamp == nil {
			services = append(services, service)
		}
	}

	slices.SortFunc(services, func(a, b *corev1.Service) int {
		return b.CreationTimestamp.Compare(a.CreationTimestamp.Time)
	})

	return services, nil
}

func (r *Reconciler) newEndpointsSyncEventHandler() *singleton.EventHandler[*ExternalProxy, *corev1.Endpoints] {
	return &singleton.EventHandler[*ExternalProxy, *corev1.Endpoints]{
		Writer:          r.Client,
		NewObject:       newEndpoints,
		ListObject:      r.listOwnedEndpoints,
		ObjectRevision:  extractExternalProxyRevision[*corev1.Endpoints],
		ActivatedObject: filter.OverrideName[*ExternalProxy, *corev1.Endpoints](getServiceName),
		ObjectPatcher: func(controller *ExternalProxy, desiredObject, activatedObject *corev1.Endpoints) (client.Patch, error) {
			object := activatedObject.DeepCopy()
			object.Subsets = desiredObject.Subsets

			return patch.CreateTwoWayMergePatch(activatedObject, applyExternalProxyRevision(controller, object))
		},
	}
}

func (r *Reconciler) listOwnedEndpoints(ctx context.Context, instance *ExternalProxy) ([]*corev1.Endpoints, error) {
	var endpointsList corev1.EndpointsList
	if err := r.listOwnedResources(ctx, &endpointsList, instance); err != nil {
		return nil, err
	}

	endpoints := make([]*corev1.Endpoints, 0, len(endpointsList.Items))
	for i := range endpointsList.Items {
		endpoint := &endpointsList.Items[i]
		if endpoint.DeletionTimestamp == nil {
			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints, nil
}

func (r *Reconciler) newIngressSyncEventHandler() *singleton.EventHandler[*ExternalProxy, *networkingv1.Ingress] {
	return &singleton.EventHandler[*ExternalProxy, *networkingv1.Ingress]{
		Writer:          r.Client,
		NewObject:       newIngress,
		ListObject:      r.listOwnedIngresses,
		ObjectRevision:  extractExternalProxyRevision[*networkingv1.Ingress],
		ActivatedObject: filter.Or(filter.SameName[*ExternalProxy, *networkingv1.Ingress], filter.First[*ExternalProxy, *networkingv1.Ingress]),
		ObjectPatcher: func(controller *ExternalProxy, desiredObject, activatedObject *networkingv1.Ingress) (client.Patch, error) {
			object := activatedObject.DeepCopy()
			object.Spec.DefaultBackend = desiredObject.Spec.DefaultBackend
			object.Spec.TLS = desiredObject.Spec.TLS
			object.Spec.Rules = desiredObject.Spec.Rules
			if desiredObject.Spec.IngressClassName != nil {
				object.Spec.IngressClassName = desiredObject.Spec.IngressClassName
			}
			utils.Merge(&object.Labels, desiredObject.Labels)
			utils.Merge(&object.Annotations, desiredObject.Annotations)

			return patch.CreateTwoWayMergePatch(activatedObject, applyExternalProxyRevision(controller, object))
		},
	}
}

func (r *Reconciler) listOwnedIngresses(ctx context.Context, instance *ExternalProxy) ([]*networkingv1.Ingress, error) {
	var ingressList networkingv1.IngressList
	if err := r.listOwnedResources(ctx, &ingressList, instance); err != nil {
		return nil, err
	}

	ingresses := make([]*networkingv1.Ingress, 0, len(ingressList.Items))
	for i := range ingressList.Items {
		endpoint := &ingressList.Items[i]
		if endpoint.DeletionTimestamp == nil {
			ingresses = append(ingresses, endpoint)
		}
	}

	slices.SortFunc(ingresses, func(a, b *networkingv1.Ingress) int {
		return b.CreationTimestamp.Compare(a.CreationTimestamp.Time)
	})

	return ingresses, nil
}

func (r *Reconciler) listOwnedResources(ctx context.Context, list client.ObjectList, instance *ExternalProxy) error {
	return r.List(ctx, list,
		client.UnsafeDisableDeepCopy,
		client.InNamespace(instance.Namespace),
		client.MatchingFields{fieldindex.IndexOwnerReferenceUID: string(instance.UID)},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ExternalProxy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Service{}, &watchEventHandler[*corev1.Service]{expectations: r.serviceSyncer}).
		Watches(&corev1.Endpoints{}, &watchEventHandler[*corev1.Endpoints]{expectations: r.endpointSyncer}).
		Watches(&networkingv1.Ingress{}, &watchEventHandler[*networkingv1.Ingress]{expectations: r.ingressSyncer}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrentReconciles,
		}).
		Complete(r)
}
