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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wjiec/mobius/api/networking"
	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
	"github.com/wjiec/mobius/internal/sync/singleton"
	"github.com/wjiec/mobius/pkg/utils"
)

type (
	ExternalProxy       = networkingv1alpha1.ExternalProxy
	ExternalProxyStatus = networkingv1alpha1.ExternalProxyStatus

	ServiceSyncer   = singleton.Singleton[*ExternalProxy, *corev1.Service]
	EndpointsSyncer = singleton.Singleton[*ExternalProxy, *corev1.Endpoints]
	IngressSyncer   = singleton.Singleton[*ExternalProxy, *networkingv1.Ingress]
)

var (
	APIVersion       = networkingv1alpha1.GroupVersion.String()
	GroupVersionKind = networkingv1alpha1.GroupVersion.WithKind("ExternalProxy")
)

// getServiceName retrieves the service name for the given ExternalProxy instance.
//
// If the service name is specified in the ExternalProxy spec, it returns that name.
// Otherwise, it returns the name of the ExternalProxy object.
func getServiceName(ep *ExternalProxy) string {
	if ep.Spec.Service.Name != "" {
		return ep.Spec.Service.Name
	}

	return ep.Name
}

// resolveControllerRef checks the controller reference of the given
// object and converts it to a controller request.
func resolveControllerRef(object metav1.Object) *ctrl.Request {
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.APIVersion == APIVersion && ownerRef.Kind == GroupVersionKind.Kind {
			return &ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: object.GetNamespace(),
					Name:      ownerRef.Name,
				},
			}
		}
	}

	return nil
}

func newService(_ context.Context, instance *ExternalProxy) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceName(instance),
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: instance.Spec.Service.Ports,
			Type:  instance.Spec.Service.Type,
		},
	}
	utils.Merge(&service.Labels, instance.Spec.Service.Labels)
	utils.Merge(&service.Annotations, instance.Spec.Service.Annotations)

	return applyExternalProxyRevision(instance, service)
}

func newEndpoints(_ context.Context, instance *ExternalProxy) *corev1.Endpoints {
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceName(instance),
			Namespace: instance.Namespace,
		},
		Subsets: make([]corev1.EndpointSubset, 0, len(instance.Spec.Backends)),
	}

	for _, backend := range instance.Spec.Backends {
		addresses := make([]corev1.EndpointAddress, 0, len(backend.Addresses))
		for _, address := range backend.Addresses {
			addresses = append(addresses, corev1.EndpointAddress{
				IP: address.IP,
			})
		}

		endpoints.Subsets = append(endpoints.Subsets, corev1.EndpointSubset{
			Addresses: addresses,
			Ports:     backend.Ports,
		})
	}

	return applyExternalProxyRevision(instance, endpoints)
}

func newIngress(_ context.Context, instance *ExternalProxy) *networkingv1.Ingress {
	if instance.Spec.Ingress == nil {
		return nil
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: instance.Spec.Ingress.IngressClassName,
		},
	}
	utils.Merge(&ingress.Labels, instance.Spec.Ingress.Labels)
	utils.Merge(&ingress.Annotations, instance.Spec.Ingress.Annotations)

	if defaultBackend := instance.Spec.Ingress.DefaultBackend; defaultBackend != nil {
		ingress.Spec.DefaultBackend = &networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: getServiceName(instance),
				Port: networkingv1.ServiceBackendPort{
					Name:   defaultBackend.Port.Name,
					Number: defaultBackend.Port.Number,
				},
			},
		}
	}

	for _, tls := range instance.Spec.Ingress.TLS {
		ingress.Spec.TLS = append(ingress.Spec.TLS, *tls.DeepCopy())
	}

	for _, rule := range instance.Spec.Ingress.Rules {
		rulePaths := make([]networkingv1.HTTPIngressPath, 0, len(rule.HTTP.Paths))
		for _, path := range rule.HTTP.Paths {
			rulePaths = append(rulePaths, networkingv1.HTTPIngressPath{
				Path:     path.Path,
				PathType: path.PathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: getServiceName(instance),
						Port: networkingv1.ServiceBackendPort{
							Name:   path.Backend.Port.Name,
							Number: path.Backend.Port.Number,
						},
					},
				},
			})
		}

		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{
			Host: rule.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: rulePaths,
				},
			},
		})
	}

	return applyExternalProxyRevision(instance, ingress)
}

// extractExternalProxyRevision retrieves the ExternalProxy revision from the object's annotations.
//
// If the annotation or parsing fails, it returns 0.
func extractExternalProxyRevision[T client.Object](object T) int64 {
	revision := object.GetAnnotations()[networking.ExternalProxyRevisionAnnotationKey]
	if revision, err := strconv.ParseInt(revision, 10, 64); err == nil {
		return revision
	}

	return 0
}

// applyExternalProxyRevision sets the revision annotation on the target object.
//
// It uses the generation of the controller object as the revision value.
func applyExternalProxyRevision[R client.Object](controller *ExternalProxy, object R) R {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[networking.ExternalProxyRevisionAnnotationKey] = strconv.FormatInt(controller.Generation, 10)
	object.SetAnnotations(annotations)

	return object
}
