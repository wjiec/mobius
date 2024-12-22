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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
)

var _ = Describe("ExternalProxy Webhook", func() {
	var (
		obj       *networkingv1alpha1.ExternalProxy
		oldObj    *networkingv1alpha1.ExternalProxy
		validator ExternalProxyCustomValidator
		defaulter ExternalProxyCustomDefaulter
	)

	BeforeEach(func() {
		obj = &networkingv1alpha1.ExternalProxy{}
		oldObj = &networkingv1alpha1.ExternalProxy{}
		validator = ExternalProxyCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")

		defaulter = ExternalProxyCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")

		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")

		objMeta := metav1.ObjectMeta{Name: "foo", Namespace: "default"}
		obj.ObjectMeta = objMeta
		oldObj.ObjectMeta = objMeta
	})

	AfterEach(func() {})

	Context("When creating ExternalProxy under Defaulting Webhook", func() {
		It("Should apply defaults when a required field is empty", func() {
			By("the name of the service is not set")
			obj.Spec.Service.Name = ""

			By("calling the Default method to apply defaults")
			Expect(defaulter.Default(ctx, obj)).To(Succeed())

			By("checking that the default values are set")
			Expect(obj.Spec.Service.Name).To(Equal(obj.Name))
		})
	})

	Context("When creating or updating ExternalProxy under Validating Webhook", func() {
		It("Should deny creation if a required field is missing", func() {
			By("simulating an invalid creation scenario")
			obj.Spec.Service.Name = ""
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})
	})

})
