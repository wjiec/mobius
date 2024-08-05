package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
)

type TestObject = networkingv1alpha1.ExternalProxy

var objects = []*TestObject{
	{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
	{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
	{ObjectMeta: metav1.ObjectMeta{Name: "baz"}},
}

func TestFirst(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		found := First(&TestObject{}, objects)
		if assert.NotNil(t, found) {
			assert.Equal(t, found.Name, "foo")
		}
	})

	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, First(&TestObject{}, []*TestObject{}))
	})
}

func TestSameName(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		found := SameName(&TestObject{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}, objects)
		if assert.NotNil(t, found) {
			assert.Equal(t, found.Name, "bar")
		}
	})

	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, SameName(&TestObject{ObjectMeta: metav1.ObjectMeta{Name: "zzz"}}, objects))
	})
}

func TestOverrideName(t *testing.T) {
	fakeExtractor := func(name string) func(*TestObject) string {
		return func(*TestObject) string { return name }
	}

	t.Run("normal", func(t *testing.T) {
		filter := OverrideName[*TestObject, *TestObject](fakeExtractor("foo"))
		found := filter(&TestObject{}, objects)
		if assert.NotNil(t, found) {
			assert.Equal(t, found.Name, "foo")
		}
	})

	t.Run("not found", func(t *testing.T) {
		filter := OverrideName[*TestObject, *TestObject](fakeExtractor("zzz"))
		assert.Nil(t, filter(&TestObject{}, objects))
	})
}

func TestOr(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		filter := Or(SameName[*TestObject, *TestObject], First[*TestObject, *TestObject])
		found := filter(&TestObject{ObjectMeta: metav1.ObjectMeta{Name: "zzz"}}, objects)
		if assert.NotNil(t, found) {
			assert.Equal(t, found.Name, "foo")
		}
	})
}
