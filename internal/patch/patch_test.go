package patch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
)

const (
	TestRevisionAnnotationKey = "patch-test-revision"
)

func TestCreateTwoWayMergePatch(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		original := &networkingv1alpha1.ExternalProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "example",
				Namespace:         "default",
				CreationTimestamp: metav1.Time{},
				Annotations: map[string]string{
					TestRevisionAnnotationKey: "1",
				},
			},
			Spec: networkingv1alpha1.ExternalProxySpec{
				Backends: []networkingv1alpha1.ExternalProxyBackend{
					{
						Addresses: []corev1.EndpointAddress{
							{IP: "192.168.1.1"},
						},
						Ports: []corev1.EndpointPort{
							{Name: "http", Port: 80},
						},
					},
				},
				Service: networkingv1alpha1.ExternalProxyService{
					Name: ptr.To("foobar"),
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 80},
					},
				},
			},
		}

		desired := original.DeepCopy()
		desired.Annotations[TestRevisionAnnotationKey] = "2"
		desired.Spec.Service.Name = nil
		desired.Spec.Ingress = &networkingv1alpha1.ExternalProxyIngress{
			Rules: []networkingv1alpha1.ExternalProxyIngressRule{
				{Host: "www.example.com"},
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

	t.Run("ingress", func(t *testing.T) {
		original := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx"),
				Rules: []networkingv1.IngressRule{
					{Host: "example.com"},
				},
			},
		}

		desired := original.DeepCopy()
		desired.Spec.IngressClassName = nil
		desired.Spec.Rules = []networkingv1.IngressRule{
			{Host: "www.example.com"},
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
		original := &networkingv1.Ingress{}
		patch, err := CreateTwoWayMergePatch(original, original)
		if assert.NoError(t, err) {
			data, err := patch.Data(original)
			if assert.NoError(t, err) {
				t.Logf("Patch(%s): %#v", patch.Type(), data)
			}
		}
	})
}
