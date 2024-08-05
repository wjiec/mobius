package expectations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewControllerExpectations(t *testing.T) {
	assert.NotNil(t, NewControllerExpectations())
}

func TestRealControllerExpectations_SatisfiedExpectations(t *testing.T) {
	e := NewControllerExpectations()

	satisfied, _ := e.SatisfiedExpectations("default/foo")
	assert.True(t, satisfied)

	e.Expect("default/foo", ActionCreations, "res1")
	e.Expect("default/foo", ActionCreations, "res2")
	satisfied, _ = e.SatisfiedExpectations("default/foo")
	assert.False(t, satisfied)

	e.Expect("default/bar", ActionDeletions, "res3")
	e.Expect("default/bar", ActionDeletions, "res4")
	satisfied, _ = e.SatisfiedExpectations("default/bar")
	assert.False(t, satisfied)

	e.Observe("default/foo", ActionCreations, "res1")
	e.Observe("default/foo", ActionCreations, "res2")
	satisfied, _ = e.SatisfiedExpectations("default/foo")
	assert.True(t, satisfied)

	e.Observe("default/bar", ActionDeletions, "res3")
	satisfied, _ = e.SatisfiedExpectations("default/bar")
	assert.False(t, satisfied)

	e.Observe("default/bar", ActionDeletions, "res4")
	satisfied, _ = e.SatisfiedExpectations("default/bar")
	assert.True(t, satisfied)
}

func TestObjectStore_Insert(t *testing.T) {
	s := newObjectStore[string, string]()
	s.Insert("foo", "value1")
	assert.True(t, s.objects["foo"].Has("value1"))

	s.Insert("foo", "value2")
	assert.True(t, s.objects["foo"].Has("value1"))
	assert.True(t, s.objects["foo"].Has("value2"))
}
