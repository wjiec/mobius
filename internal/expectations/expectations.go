package expectations

import (
	"context"
	"flag"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	ExpectationTimeout time.Duration
)

func init() {
	flag.DurationVar(&ExpectationTimeout, "expectation-timeout", 5*time.Minute, "The expectation timeout. Defaults 5min")
}

type ControllerAction string

const (
	ActionCreations ControllerAction = "creations"
	ActionDeletions ControllerAction = "deletions"
)

type ControllerKey = string
type controllerKeyHolder struct{}

func WithControllerKey(ctx context.Context, key ControllerKey) context.Context {
	return context.WithValue(ctx, controllerKeyHolder{}, key)
}

func ControllerKeyFromCtx(ctx context.Context) ControllerKey {
	if key, ok := ctx.Value(controllerKeyHolder{}).(ControllerKey); ok {
		return key
	}
	return ""
}

type ControllerExpectations interface {
	Expect(key ControllerKey, action ControllerAction, name string)
	Observe(key ControllerKey, action ControllerAction, name string)
	SatisfiedExpectations(key ControllerKey) (satisfied bool, unsatisfiedDuration time.Duration)
}

func NewControllerExpectations() ControllerExpectations {
	return &realControllerExpectations{
		cache: make(map[ControllerKey]*objectStore[ControllerAction, string]),
	}
}

type realControllerExpectations struct {
	sync.Mutex

	cache map[ControllerKey]*objectStore[ControllerAction, string]
}

func (r *realControllerExpectations) Expect(key ControllerKey, action ControllerAction, name string) {
	r.Lock()
	defer r.Unlock()

	expectations := r.cache[key]
	if expectations == nil {
		expectations = newObjectStore[ControllerAction, string]()
		r.cache[key] = expectations
	}

	expectations.Insert(action, name)
}

func (r *realControllerExpectations) Observe(key ControllerKey, action ControllerAction, name string) {
	r.Lock()
	defer r.Unlock()

	expectations := r.cache[key]
	if expectations == nil {
		return
	}

	if s, ok := expectations.objects[action]; ok {
		s.Delete(name)

		for _, elem := range expectations.objects {
			if elem.Len() > 0 {
				return
			}
		}
		delete(r.cache, key)
	}
}

func (r *realControllerExpectations) SatisfiedExpectations(key ControllerKey) (bool, time.Duration) {
	r.Lock()
	defer r.Unlock()

	expectations := r.cache[key]
	if expectations == nil {
		return true, time.Duration(0)
	}

	for _, elem := range expectations.objects {
		if elem.Len() > 0 {
			if expectations.firstUnsatisfied.IsZero() {
				expectations.firstUnsatisfied = time.Now()
			}
			return false, time.Since(expectations.firstUnsatisfied)
		}
	}

	delete(r.cache, key)
	return true, time.Duration(0)
}

type objectStore[K comparable, V comparable] struct {
	objects          map[K]sets.Set[V]
	firstUnsatisfied time.Time
}

func newObjectStore[K comparable, V comparable]() *objectStore[K, V] {
	return &objectStore[K, V]{
		objects: make(map[K]sets.Set[V]),
	}
}

func (o *objectStore[K, V]) Insert(action K, value V) {
	if set, ok := o.objects[action]; !ok {
		o.objects[action] = sets.New(value)
	} else {
		set.Insert(value)
	}
}
