package patch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	TestRevisionAnnotationKey = "patch-test-revision"
)

func TestCreateTwoWayMergePatch(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		original := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
				Annotations: map[string]string{
					TestRevisionAnnotationKey: "1",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "nginx:alpine",
					},
					{
						Name:  "proxy",
						Image: "nginx:alpine",
					},
				},
			},
		}

		desired := original.DeepCopy()
		desired.Annotations[TestRevisionAnnotationKey] = "2"
		desired.Spec.Containers[0].Name = "web"
		desired.Spec.InitContainers = []corev1.Container{
			{
				Name:  "sysctl",
				Image: "alpine:latest",
				SecurityContext: &corev1.SecurityContext{
					Privileged: ptr.To(true),
				},
			},
		}

		patch, err := CreateTwoWayMergePatch(original, desired)
		if assert.NoError(t, err) {
			data, err := patch.Data(original)
			if assert.NoError(t, err) {
				t.Logf("Patch(%s): %s", patch.Type(), data)
			}
		}
	})

	t.Run("unchanged", func(t *testing.T) {
		original := &corev1.Pod{}
		patch, err := CreateTwoWayMergePatch(original, original)
		if assert.NoError(t, err) {
			data, err := patch.Data(original)
			if assert.NoError(t, err) {
				t.Logf("Patch(%s): %s", patch.Type(), data)
			}
		}
	})
}
