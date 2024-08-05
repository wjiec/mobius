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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/wjiec/mobius/api/networking"
	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
)

var _ = Describe("ExternalProxy Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()
		namespacedName := types.NamespacedName{
			Name:      "test-externalproxy",
			Namespace: "default",
		}

		BeforeEach(func() {
			instance := &networkingv1alpha1.ExternalProxy{}

			By("creating the custom resource for the Kind ExternalProxy")
			err := k8sClient.Get(ctx, namespacedName, instance)
			if Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred()) {
				instance = &networkingv1alpha1.ExternalProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      namespacedName.Name,
						Namespace: namespacedName.Namespace,
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
							Name: ptr.To("example"),
							Ports: []corev1.ServicePort{
								{Name: "http", Port: 80},
							},
						},
						Ingress: &networkingv1alpha1.ExternalProxyIngress{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"nginx.ingress.kubernetes.io/client-max-body-size": "100m",
								},
							},
							TLS: []networkingv1.IngressTLS{
								{
									Hosts:      []string{"foo.laboys.org"},
									SecretName: "star-laboys-org",
								},
							},
							Rules: []networkingv1alpha1.ExternalProxyIngressRule{
								{
									Host: "foo.laboys.org",
									HTTP: &networkingv1alpha1.ExternalProxyIngressHttpRuleValue{
										Paths: []networkingv1alpha1.ExternalProxyIngressHttpPath{
											{
												Path:     "/",
												PathType: ptr.To(networkingv1.PathTypeImplementationSpecific),
												Backend: &networkingv1alpha1.ExternalProxyIngressBackend{
													Port: networkingv1alpha1.ExternalProxyServiceBackendPort{
														Name: "http",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			}
		})

		AfterEach(func() {
			instance := &networkingv1alpha1.ExternalProxy{}
			err := k8sClient.Get(ctx, namespacedName, instance)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific instance instance ExternalProxy")
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			reconciler := NewExternalProxyReconciler(k8sClient, k8sClient.Scheme())
			Expect(reconciler).NotTo(BeNil())

			err := reconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the created resource")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				var instance networkingv1alpha1.ExternalProxy
				if err := k8sClient.Get(ctx, namespacedName, &instance); err != nil {
					return false
				}

				return instance.Status.ServiceName != ""
			}).Should(BeTrue())

			By("refresh the ExternalProxy resource")
			resource := &networkingv1alpha1.ExternalProxy{}
			err = k8sClient.Get(ctx, namespacedName, resource)
			if Expect(err).NotTo(HaveOccurred()) {
				Expect(resource.Status.Ready).Should(BeTrue())
				Expect(resource.Status.ServiceName).Should(Equal(getServiceName(resource)))
			}

			By("validating Service resources for ExternalProxy")
			service := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: resource.Namespace, Name: resource.Status.ServiceName}, service)
			if Expect(err).NotTo(HaveOccurred()) {
				Expect(service.Spec.Ports).Should(HaveLen(1))
			}

			By("validating Endpoints resources for ExternalProxy")
			endpoint := &corev1.Endpoints{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: resource.Namespace, Name: resource.Status.ServiceName}, endpoint)
			if Expect(err).NotTo(HaveOccurred()) {
				Expect(endpoint.Subsets).Should(HaveLen(1))
			}

			By("validating Ingress resources for ExternalProxy")
			ingress := &networkingv1.Ingress{}
			err = k8sClient.Get(ctx, namespacedName, ingress)
			if Expect(err).NotTo(HaveOccurred()) {
				Expect(ingress.Annotations).Should(HaveKey(networking.ExternalProxyRevisionAnnotationKey))
				Expect(ingress.Annotations).Should(HaveKey("nginx.ingress.kubernetes.io/client-max-body-size"))
				Expect(ingress.Spec.TLS).Should(HaveLen(1))
				Expect(ingress.Spec.Rules).Should(HaveLen(1))
			}
		})
	})
})
