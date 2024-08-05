package singleton

import (
	"context"
	"reflect"

	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wjiec/mobius/internal/expectations"
)

type Singleton[C, R client.Object] struct {
	expectations.ControllerExpectations

	scheme       *runtime.Scheme
	eventHandler *EventHandler[C, R]
}

// New creates a new instance of Singleton with the given scheme and event handler.
func New[C, R client.Object](scheme *runtime.Scheme, eventHandler *EventHandler[C, R]) *Singleton[C, R] {
	return &Singleton[C, R]{
		ControllerExpectations: expectations.NewControllerExpectations(),
		scheme:                 scheme,
		eventHandler:           eventHandler,
	}
}

// EventHandler encapsulates the operations related to object lifecycle events for the Singleton.
type EventHandler[C, R client.Object] struct {
	client.Writer

	NewObject  func(ctx context.Context, controller C) R
	ListObject func(ctx context.Context, controller C) ([]R, error)

	ActivatedObject func(controller C, objects []R) R
	ObjectRevision  func(Object R) int64
	ObjectPatcher   func(controller C, desiredObject, activatedObject R) (client.Patch, error)
}

// Sync ensures the objects are in sync with the expected state.
func (s *Singleton[C, R]) Sync(ctx context.Context, controller C) error {
	// Retrieve objects associated with the controller.
	ownerObjects, err := s.eventHandler.ListObject(ctx, controller)
	if err != nil {
		return err
	}

	desiredObject := s.eventHandler.NewObject(ctx, controller)
	// If no such object currently exists, and we can create this object
	if len(ownerObjects) == 0 && !reflect.ValueOf(desiredObject).IsZero() {
		if err = ctrl.SetControllerReference(controller, desiredObject, s.scheme); err != nil {
			return err
		}

		s.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, desiredObject.GetName())
		if err = s.eventHandler.Create(ctx, desiredObject); err != nil {
			s.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, desiredObject.GetName())
			return err
		}

		return nil
	}

	// Determine the activated object from the object list.
	var activatedObject R
	var hasActivatedObject bool
	if len(ownerObjects) != 0 {
		activatedObject = s.eventHandler.ActivatedObject(controller, ownerObjects)
		hasActivatedObject = !reflect.ValueOf(activatedObject).IsZero()
	}

	// Delete all objects that are inactivated.
	var eg multierror.Group
	for _, currObject := range ownerObjects {
		if !hasActivatedObject || currObject.GetUID() != activatedObject.GetUID() {
			eg.Go(func() error { return s.eventHandler.Delete(ctx, currObject) })
		}
	}
	if err = eg.Wait().ErrorOrNil(); err != nil {
		return err
	}

	// Check if the revision of the activated object matches the controller's generation.
	if hasActivatedObject && s.eventHandler.ObjectRevision(activatedObject) != controller.GetGeneration() {
		patch, err := s.eventHandler.ObjectPatcher(controller, desiredObject, activatedObject)
		if err != nil {
			return err
		}

		return s.eventHandler.Patch(ctx, activatedObject, patch)
	}

	return nil
}
