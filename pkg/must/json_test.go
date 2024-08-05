package must

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestJsonMarshal(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		assert.NotPanics(t, func() {
			JsonMarshal(&corev1.Pod{})
		})
	})

	t.Run("nil", func(t *testing.T) {
		assert.NotPanics(t, func() {
			JsonMarshal[*corev1.Pod](nil)
		})
	})

	t.Run("panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			JsonMarshal(struct{ unexported int }{})
		})
	})
}
