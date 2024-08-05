package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMerge(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		m := map[string]string{"foo": "foo"}
		Merge(&m, map[string]string{"foo": "bar", "say": "hello"})

		assert.Equal(t, m["foo"], "bar")
		assert.Equal(t, m["say"], "hello")
	})

	t.Run("nil", func(t *testing.T) {
		var m map[string]string
		if assert.Nil(t, m) {
			Merge(&m, map[string]string{"foo": "bar", "say": "hello"})

			if assert.NotNil(t, m) {
				assert.Equal(t, m["foo"], "bar")
				assert.Equal(t, m["say"], "hello")
			}
		}
	})
}
