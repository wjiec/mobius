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
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	netutils "k8s.io/utils/net"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/wjiec/mobius/api/networking/v1alpha1"
)

var (
	allowedTemplateObjectMetaFields = map[string]bool{
		"Name":        true,
		"Annotations": true,
		"Labels":      true,
	}
)

// SetupExternalProxyWebhookWithManager registers the webhook for ExternalProxy in the manager.
func SetupExternalProxyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&networkingv1alpha1.ExternalProxy{}).
		WithValidator(&ExternalProxyCustomValidator{}).
		WithDefaulter(&ExternalProxyCustomDefaulter{
			DefaultServiceType:         corev1.ServiceTypeClusterIP,
			DefaultBackendPortProtocol: corev1.ProtocolTCP,
			DefaultIngressPathType:     networkingv1.PathTypeImplementationSpecific,
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-networking-laboys-org-v1alpha1-externalproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=networking.laboys.org,resources=externalproxies,verbs=create;update,versions=v1alpha1,name=mexternalproxy-v1alpha1.kb.io,admissionReviewVersions=v1

// ExternalProxyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ExternalProxy when those are created or updated.
type ExternalProxyCustomDefaulter struct {
	DefaultServiceType         corev1.ServiceType
	DefaultBackendPortProtocol corev1.Protocol
	DefaultIngressPathType     networkingv1.PathType
}

var _ webhook.CustomDefaulter = &ExternalProxyCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ExternalProxy.
func (d *ExternalProxyCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	logger := logf.FromContext(ctx).WithName("externalproxy-defaulter")

	instance, ok := obj.(*networkingv1alpha1.ExternalProxy)
	if !ok {
		return fmt.Errorf("expected an ExternalProxy object but got %T", obj)
	}
	logger.Info("Defaulting for ExternalProxy", "name", instance.GetName())

	d.applyDefaults(instance)
	return nil
}

// applyDefaults applies default values to ExternalProxy fields.
func (d *ExternalProxyCustomDefaulter) applyDefaults(ep *networkingv1alpha1.ExternalProxy) {
	if len(ep.Spec.Service.Name) == 0 {
		ep.Spec.Service.Name = ep.Name
	}
	if len(ep.Spec.Service.Type) == 0 {
		ep.Spec.Service.Type = d.DefaultServiceType
	}

	noServicePorts := len(ep.Spec.Service.Ports) == 0
	for i := range ep.Spec.Backends {
		backend := &ep.Spec.Backends[i]
		for port := range backend.Ports {
			if len(backend.Ports[port].Protocol) == 0 {
				backend.Ports[port].Protocol = d.DefaultBackendPortProtocol
			}

			// If the port is not specified in the Service object, then we copy
			// the port information from the first Backend into the Service.
			if noServicePorts && i == 0 {
				ep.Spec.Service.Ports = append(ep.Spec.Service.Ports, corev1.ServicePort{
					Name:        backend.Ports[port].Name,
					Protocol:    backend.Ports[port].Protocol,
					AppProtocol: backend.Ports[port].AppProtocol,
					Port:        backend.Ports[port].Port,
					TargetPort:  preferredTargetPort(&backend.Ports[port]),
				})
			}
		}
	}

	if ingress := ep.Spec.Ingress; ingress != nil {
		if len(ingress.Name) == 0 {
			ingress.Name = ep.Name
		}

		// If there is only one port, then we can generate default HTTP traffic rules for it
		if len(ep.Spec.Service.Ports) == 1 && len(ingress.Rules) != 0 {
			var ingressBackendServicePort networkingv1alpha1.ExternalProxyServiceBackendPort
			// If we can use the name that's best, otherwise we'll have to use the port number
			// to specify the destination of the traffic.
			if len(ep.Spec.Service.Ports[0].Name) != 0 {
				ingressBackendServicePort.Name = ep.Spec.Service.Ports[0].Name
			} else {
				ingressBackendServicePort.Number = ep.Spec.Service.Ports[0].Port
			}

			for i := range ingress.Rules {
				rule := &ingress.Rules[i]
				if rule.HTTP == nil {
					rule.HTTP = &networkingv1alpha1.ExternalProxyIngressHttpRuleValue{
						Paths: []networkingv1alpha1.ExternalProxyIngressHttpPath{
							{Path: "/"},
						},
					}
				}

				if len(rule.HTTP.Paths) != 0 {
					for path := range rule.HTTP.Paths {
						if rule.HTTP.Paths[path].PathType == nil {
							rule.HTTP.Paths[path].PathType = ptr.To(d.DefaultIngressPathType)
						}

						if rule.HTTP.Paths[path].Backend == nil {
							rule.HTTP.Paths[path].Backend = &networkingv1alpha1.ExternalProxyIngressBackend{
								Port: ingressBackendServicePort,
							}
						}
					}
				}
			}
		}
	}
}

func preferredTargetPort(port *corev1.EndpointPort) intstr.IntOrString {
	if len(port.Name) != 0 {
		return intstr.FromString(port.Name)
	}
	return intstr.FromInt32(port.Port)
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-networking-laboys-org-v1alpha1-externalproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=networking.laboys.org,resources=externalproxies,verbs=create;update,versions=v1alpha1,name=vexternalproxy-v1alpha1.kb.io,admissionReviewVersions=v1

// ExternalProxyCustomValidator struct is responsible for validating the ExternalProxy resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ExternalProxyCustomValidator struct{}

var _ webhook.CustomValidator = &ExternalProxyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ExternalProxy.
func (v *ExternalProxyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := logf.FromContext(ctx).WithName("externalproxy-validator")

	instance, ok := obj.(*networkingv1alpha1.ExternalProxy)
	if !ok {
		return nil, fmt.Errorf("expected a ExternalProxy object but got %T", obj)
	}
	logger.Info("Validation for ExternalProxy upon creation", "name", instance.GetName())

	return nil, v.validateExternalProxy(instance)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ExternalProxy.
func (v *ExternalProxyCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logger := logf.FromContext(ctx).WithName("externalproxy-validator")

	instance, ok := newObj.(*networkingv1alpha1.ExternalProxy)
	if !ok {
		return nil, fmt.Errorf("expected a ExternalProxy object for the newObj but got %T", newObj)
	}
	logger.Info("Validation for ExternalProxy upon update", "name", instance.GetName())

	return nil, v.validateExternalProxy(instance)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ExternalProxy.
func (v *ExternalProxyCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateExternalProxy validates the fields of a ExternalProxy object.
func (v *ExternalProxyCustomValidator) validateExternalProxy(instance *networkingv1alpha1.ExternalProxy) error {
	allErrors := validateExternalProxySpec(&instance.Spec, field.NewPath("spec"))

	if len(allErrors) != 0 {
		return apierrors.NewInvalid(
			networkingv1alpha1.GroupVersion.WithKind("ExternalProxy").GroupKind(),
			instance.Name, allErrors)
	}
	return nil
}

func validateExternalProxySpec(spec *networkingv1alpha1.ExternalProxySpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	allPortNames := sets.New[string]()
	for _, port := range spec.Service.Ports {
		allPortNames.Insert(port.Name)
	}

	allErrors = append(allErrors, validateExternalProxyBackends(spec.Backends, allPortNames, path.Child("backends"))...)
	allErrors = append(allErrors, validateExternalProxyService(&spec.Service, path.Child("service"))...)
	if spec.Ingress != nil {
		opts := IngressValidationOptions{
			AllowInvalidSecretName:       false,
			AllowInvalidWildcardHostRule: false,
		}
		allErrors = append(allErrors, validateExternalProxyIngress(spec.Ingress, path.Child("ingress"), opts)...)
	}

	return allErrors
}

func validateExternalProxyBackends(backends []networkingv1alpha1.ExternalProxyBackend, allPortNames sets.Set[string], path *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	if len(backends) == 0 {
		allErrors = append(allErrors, field.Required(path, "must be specified"))
	}

	for i := range backends {
		backend := &backends[i]
		idxPath := path.Index(i)

		if len(backend.Addresses) == 0 {
			allErrors = append(allErrors, field.Required(idxPath, "must specify at least one `address`"))
		}
		if len(backend.Ports) == 0 {
			allErrors = append(allErrors, field.Required(idxPath, "must specify at least one `port`"))
		}

		for addr := range backend.Addresses {
			allErrors = append(allErrors, validateExternalProxyBackendAddress(&backend.Addresses[addr], idxPath.Child("addresses").Index(addr))...)
		}

		requireName := len(backend.Ports) > 1
		for port := range backend.Ports {
			portPath := idxPath.Child("ports").Index(port)
			if requireName && !allPortNames.Has(backend.Ports[port].Name) {
				allErrors = append(allErrors, field.Invalid(portPath.Child("name"), backend.Ports[port].Name, "should match the port in the Service"))
			}
			allErrors = append(allErrors, validateEndpointPort(&backend.Ports[port], requireName, portPath)...)
		}
	}

	return allErrors
}

func validateExternalProxyBackendAddress(address *networkingv1alpha1.ExternalProxyBackendAddress, path *field.Path) field.ErrorList {
	return ValidateNonSpecialIP(address.IP, path.Child("ip"))
}

func validateExternalProxyService(svc *networkingv1alpha1.ExternalProxyService, path *field.Path) field.ErrorList {
	allErrors := validateExternalProxySubObjectMetadata(&svc.ObjectMeta, apimachineryvalidation.NameIsDNS1035Label, path.Child("metadata"))

	// If the user does not specify the port of the Service, we will automatically populate
	// the Service's port information in Defaulter.
	if len(svc.Ports) == 0 {
		allErrors = append(allErrors, field.Required(path.Child("ports"), "must specify at least one `port`"))
	}

	requireName := len(svc.Ports) > 1
	allPortNames := sets.Set[string]{}
	for i := range svc.Ports {
		portPath := path.Child("ports").Index(i)
		allErrors = append(allErrors, validateExternalProxyServicePort(&svc.Ports[i], requireName, &allPortNames, portPath)...)
	}

	return allErrors
}

func validateExternalProxySubObjectMetadata(objectMeta *metav1.ObjectMeta, nameFn apimachineryvalidation.ValidateNameFunc, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	allErrors = append(allErrors, apimachineryvalidation.ValidateObjectMeta(objectMeta, false, nameFn, path)...)
	allErrors = append(allErrors, apimachineryvalidation.ValidateAnnotations(objectMeta.Annotations, path.Child("annotations"))...)
	allErrors = append(allErrors, unversionedvalidation.ValidateLabels(objectMeta.Labels, path.Child("labels"))...)
	// All other fields are not supported and thus must not be set
	// to avoid confusion.  We could reject individual fields,
	// but then adding a new one to ObjectMeta wouldn't be checked
	// unless this code gets updated. Instead, we ensure that
	// only allowed fields are set via reflection.
	allErrors = append(allErrors, validateFieldAllowList(*objectMeta, allowedTemplateObjectMetaFields, "cannot be set", path)...)

	return allErrors
}

func validateExternalProxyServicePort(sp *corev1.ServicePort, requireName bool, allPortName *sets.Set[string], path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if requireName && len(sp.Name) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("name"), ""))
	} else if len(sp.Name) != 0 {
		allErrs = append(allErrs, ValidateDNS1123Label(sp.Name, path.Child("name"))...)
		if allPortName.Has(sp.Name) {
			allErrs = append(allErrs, field.Duplicate(path.Child("name"), sp.Name))
		} else {
			allPortName.Insert(sp.Name)
		}
	}

	for _, msg := range validation.IsValidPortNum(int(sp.Port)) {
		allErrs = append(allErrs, field.Invalid(path.Child("port"), sp.Port, msg))
	}

	if len(sp.Protocol) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("protocol"), ""))
	} else if !supportedPortProtocols.Has(sp.Protocol) {
		allErrs = append(allErrs, field.NotSupported(path.Child("protocol"), sp.Protocol, sets.List(supportedPortProtocols)))
	}

	allErrs = append(allErrs, ValidatePortNumOrName(sp.TargetPort, path.Child("targetPort"))...)

	if sp.AppProtocol != nil {
		allErrs = append(allErrs, ValidateQualifiedName(*sp.AppProtocol, path.Child("appProtocol"))...)
	}

	return allErrs
}

func validateExternalProxyIngress(ing *networkingv1alpha1.ExternalProxyIngress, path *field.Path, opts IngressValidationOptions) field.ErrorList {
	var allErrors field.ErrorList
	allErrors = append(allErrors, validateExternalProxySubObjectMetadata(&ing.ObjectMeta, apimachineryvalidation.NameIsDNSSubdomain, path.Child("metadata"))...)

	if ing.IngressClassName != nil {
		for _, msg := range apimachineryvalidation.NameIsDNSSubdomain(*ing.IngressClassName, false) {
			allErrors = append(allErrors, field.Invalid(path.Child("ingressClassName"), *ing.IngressClassName, msg))
		}
	}

	if ing.DefaultBackend != nil {
		allErrors = append(allErrors, validateExternalProxyIngressBackend(ing.DefaultBackend, path.Child("defaultBackend"))...)
	}

	if len(ing.Rules) == 0 && ing.DefaultBackend == nil {
		allErrors = append(allErrors, field.Invalid(path, ing.Rules,
			"either `defaultBackend` or `rules` must be specified"))
	}

	if len(ing.TLS) > 0 {
		allErrors = append(allErrors, validateIngressTLS(ing.TLS, path.Child("tls"), opts)...)
	}

	if len(ing.Rules) != 0 {
		allErrors = append(allErrors, validateExternalProxyIngressRule(ing.Rules, path.Child("rules"), opts)...)
	}

	return allErrors
}

func validateExternalProxyIngressRule(rule []networkingv1alpha1.ExternalProxyIngressRule, path *field.Path, opts IngressValidationOptions) field.ErrorList {
	var allErrors field.ErrorList
	for i, elem := range rule {
		wildcardHost := false
		if len(elem.Host) > 0 {
			if isIP := netutils.ParseIPSloppy(elem.Host) != nil; isIP {
				allErrors = append(allErrors, field.Invalid(path.Index(i).Child("host"), elem.Host, "must be a DNS name, not an IP address"))
			}

			if strings.Contains(elem.Host, "*") {
				for _, msg := range validation.IsWildcardDNS1123Subdomain(elem.Host) {
					allErrors = append(allErrors, field.Invalid(path.Index(i).Child("host"), elem.Host, msg))
				}
				wildcardHost = true
			} else {
				for _, msg := range validation.IsDNS1123Subdomain(elem.Host) {
					allErrors = append(allErrors, field.Invalid(path.Index(i).Child("host"), elem.Host, msg))
				}
			}
		}

		if !wildcardHost || !opts.AllowInvalidWildcardHostRule {
			allErrors = append(allErrors, validateExternalProxyIngressHttpRuleValue(elem.HTTP, path.Index(i))...)
		}
	}

	return allErrors
}

func validateExternalProxyIngressHttpRuleValue(value *networkingv1alpha1.ExternalProxyIngressHttpRuleValue, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	if value != nil {
		if len(value.Paths) == 0 {
			allErrors = append(allErrors, field.Required(path.Child("paths"), ""))
		}

		for i := range value.Paths {
			allErrors = append(allErrors, validateExternalProxyIngressHttpPath(&value.Paths[i], path.Child("paths"))...)
		}
	}

	return allErrors
}

var (
	supportedPathTypes = sets.NewString(
		string(networkingv1.PathTypeExact),
		string(networkingv1.PathTypePrefix),
		string(networkingv1.PathTypeImplementationSpecific),
	)
	invalidPathSequences = []string{"//", "/./", "/../", "%2f", "%2F"}
	invalidPathSuffixes  = []string{"/..", "/."}
)

func validateExternalProxyIngressHttpPath(hp *networkingv1alpha1.ExternalProxyIngressHttpPath, path *field.Path) field.ErrorList {
	allErrors := field.ErrorList{}

	if hp.PathType == nil {
		return append(allErrors, field.Required(path.Child("pathType"), "pathType must be specified"))
	}

	switch *hp.PathType {
	case networkingv1.PathTypeExact, networkingv1.PathTypePrefix:
		if !strings.HasPrefix(hp.Path, "/") {
			allErrors = append(allErrors, field.Invalid(path.Child("path"), hp.Path, "must be an absolute path"))
		}
		if len(hp.Path) > 0 {
			for _, invalidSeq := range invalidPathSequences {
				if strings.Contains(hp.Path, invalidSeq) {
					allErrors = append(allErrors, field.Invalid(path.Child("path"), hp.Path, fmt.Sprintf("must not contain '%s'", invalidSeq)))
				}
			}

			for _, invalidSuff := range invalidPathSuffixes {
				if strings.HasSuffix(hp.Path, invalidSuff) {
					allErrors = append(allErrors, field.Invalid(path.Child("path"), hp.Path, fmt.Sprintf("cannot end with '%s'", invalidSuff)))
				}
			}
		}
	case networkingv1.PathTypeImplementationSpecific:
		if len(hp.Path) > 0 {
			if !strings.HasPrefix(hp.Path, "/") {
				allErrors = append(allErrors, field.Invalid(path.Child("path"), hp.Path, "must be an absolute path"))
			}
		}
	default:
		allErrors = append(allErrors, field.NotSupported(path.Child("pathType"), *hp.PathType, supportedPathTypes.List()))
	}

	allErrors = append(allErrors, validateExternalProxyIngressBackend(hp.Backend, path.Child("backend"))...)
	return allErrors
}

func validateExternalProxyIngressBackend(be *networkingv1alpha1.ExternalProxyIngressBackend, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	hasPortName := len(be.Port.Name) > 0
	hasPortNumber := be.Port.Number != 0
	if hasPortName && hasPortNumber {
		allErrors = append(allErrors, field.Invalid(path, "", "cannot set both port name & port number"))
	} else if hasPortName {
		for _, msg := range validation.IsValidPortName(be.Port.Name) {
			allErrors = append(allErrors, field.Invalid(path.Child("service", "port", "name"), be.Port.Name, msg))
		}
	} else if hasPortNumber {
		for _, msg := range validation.IsValidPortNum(int(be.Port.Number)) {
			allErrors = append(allErrors, field.Invalid(path.Child("service", "port", "number"), be.Port.Number, msg))
		}
	} else {
		allErrors = append(allErrors, field.Required(path, "port name or number is required"))
	}

	return allErrors
}
