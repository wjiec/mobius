package networking

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
)

type StatusUpdater interface {
	UpdateStatus(ctx context.Context, ep *ExternalProxy, newStatus *networkingv1alpha1.ExternalProxyStatus) error
}

func NewStatusUpdater(c client.Client) StatusUpdater {
	return &realStatusUpdater{Client: c}
}

type realStatusUpdater struct {
	client.Client
}

func (r *realStatusUpdater) UpdateStatus(ctx context.Context, ep *ExternalProxy, newStatus *networkingv1alpha1.ExternalProxyStatus) error {
	if reflect.DeepEqual(&ep.Status, newStatus) {
		return nil
	}

	log.FromContext(ctx).Info("Update ExternalProxy status", "serviceName", newStatus.ServiceName, "ready", newStatus.Ready)
	return r.updateStatus(ctx, client.ObjectKeyFromObject(ep), newStatus)
}

func (r *realStatusUpdater) updateStatus(ctx context.Context, namespaceName types.NamespacedName, newStatus *networkingv1alpha1.ExternalProxyStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var instance ExternalProxy
		if err := r.Get(ctx, namespaceName, &instance); err != nil {
			return err
		}

		instance.Status = *newStatus
		return r.Status().Update(ctx, &instance)
	})
}
