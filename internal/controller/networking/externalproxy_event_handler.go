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
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/wjiec/mobius/internal/expectations"
)

var (
	// initialingRateLimiter calculates the delay duration for existing resources
	// triggered Create event when the Informer cache has just synced.
	initialingRateLimiter = workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](3*time.Second, 30*time.Second)
)

type watchEventHandler[T client.Object] struct {
	expectations expectations.ControllerExpectations
}

func (w *watchEventHandler[T]) Create(ctx context.Context, evt event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	logger := log.FromContext(ctx)
	if evt.Object.GetDeletionTimestamp() != nil {
		w.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.Object}, q)
		return
	}

	req := resolveControllerRef(evt.Object)
	if req == nil {
		return
	}

	logger.V(4).Info("Created", "obj", klog.KObj(evt.Object), "owner", req)
	isSatisfied, _ := w.expectations.SatisfiedExpectations(req.String())
	w.expectations.Observe(req.String(), expectations.ActionCreations, evt.Object.GetName())
	if isSatisfied {
		// If the expectation is satisfied, it should be an existing Pod and the Informer
		// cache should have just synced.
		q.AddAfter(*req, initialingRateLimiter.When(*req))
	} else {
		// Otherwise, add it immediately and reset the rate limiter
		initialingRateLimiter.Forget(*req)
		q.Add(*req)
	}
}

func (w *watchEventHandler[T]) Update(ctx context.Context, evt event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	if evt.ObjectNew.GetDeletionTimestamp() != nil {
		w.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.ObjectNew}, q)
		return
	}
}

func (w *watchEventHandler[T]) Delete(ctx context.Context, evt event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[ctrl.Request]) {
	logger := log.FromContext(ctx)
	if _, ok := evt.Object.(T); !ok {
		logger.Error(nil, "Skipped deletion event", "deleteStateUnknown", evt.DeleteStateUnknown, "obj", evt.Object)
		return
	}

	req := resolveControllerRef(evt.Object)
	if req == nil {
		return
	}

	w.expectations.Observe(req.String(), expectations.ActionDeletions, evt.Object.GetName())
	q.Add(*req)
}

func (w *watchEventHandler[T]) Generic(context.Context, event.TypedGenericEvent[client.Object], workqueue.TypedRateLimitingInterface[ctrl.Request]) {
}
