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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalProxySpec defines the desired state of ExternalProxy
type ExternalProxySpec struct {
	Backends []ExternalProxyBackend `json:"backends"`
	Service  ExternalProxyService   `json:"service,omitempty"`
	Ingress  *ExternalProxyIngress  `json:"ingress,omitempty"`
}

type ExternalProxyBackend struct {
	Addresses []corev1.EndpointAddress `json:"addresses"`
	Ports     []corev1.EndpointPort    `json:"ports"`
}

type ExternalProxyService struct {
	// Standard object's metadata.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The name of the Service, if empty the name of the ExternalProxy is used.
	Name *string `json:"name,omitempty"`

	// type determines how the Service is exposed. Defaults to ClusterIP.
	Type corev1.ServiceType `json:"type"`

	// The list of ports that are exposed by this service.
	Ports []corev1.ServicePort `json:"ports"`
}

type ExternalProxyIngress struct {
	// Standard object's metadata.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ingressClassName is the name of an IngressClass cluster resource.
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// defaultBackend is the backend that should handle requests that don't
	// match any rule. If Rules are not specified, DefaultBackend must be specified.
	// If DefaultBackend is not set, the handling of requests that do not match any
	// of the rules will be up to the Ingress controller.
	// +optional
	DefaultBackend *ExternalProxyIngressBackend `json:"defaultBackend,omitempty"`

	// tls represents the TLS configuration. Currently, the Ingress only supports a
	// single TLS port, 443. If multiple members of this list specify different hosts,
	// they will be multiplexed on the same port according to the hostname specified
	// through the SNI TLS extension, if the ingress controller fulfilling the
	// ingress supports SNI.
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`

	// rules is a list of host rules used to configure the Ingress. If unspecified,
	// or no rule matches, all traffic is sent to the default backend.
	Rules []ExternalProxyIngressRule `json:"rules,omitempty"`
}

type ExternalProxyIngressMetadata struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// ExternalProxyIngressRule represents the rules mapping the paths under a specified
// host to the related backend services. Incoming requests are first evaluated for a
// host match, then routed to the backend associated with the matching IngressRuleValue.
type ExternalProxyIngressRule struct {
	// host is the fully qualified domain name of a network host, as defined by RFC 3986.
	//
	// Incoming requests are matched against the host before the IngressRuleValue. If the
	// host is unspecified, the Ingress routes all traffic based on the specified IngressRuleValue.
	Host string `json:"host,omitempty"`

	// http represents a rule to route requests for this ExternalProxyIngressRule.
	//
	// If unspecified, the rule defaults to a http catch-all. Whether that sends
	// just traffic matching the host to the default backend or all traffic to the
	// default backend, is left to the controller fulfilling the Ingress.
	HTTP *ExternalProxyIngressHttpRuleValue `json:"http,omitempty"`
}

// ExternalProxyIngressHttpRuleValue is a list of http selectors pointing to backends.
type ExternalProxyIngressHttpRuleValue struct {
	// paths is a collection of paths that map requests to backends.
	Paths []ExternalProxyIngressHttpPath `json:"paths"`
}

// ExternalProxyIngressHttpPath associates a path with a backend. Incoming urls matching
// the path are forwarded to the backend.
type ExternalProxyIngressHttpPath struct {
	// path is matched against the path of an incoming request. Currently, it can
	// contain characters disallowed from the conventional "path" part of a URL
	// as defined by RFC 3986. Paths must begin with a '/' and must be present
	// when using PathType with value "Exact" or "Prefix".
	Path string `json:"path,omitempty"`

	// pathType determines the interpretation of the path matching.
	PathType *networkingv1.PathType `json:"pathType"`

	// backend defines the referenced service endpoint to which the traffic
	// will be forwarded to.
	Backend *ExternalProxyIngressBackend `json:"backend,omitempty"`
}

// ExternalProxyIngressBackend describes all endpoints for a given service and port.
type ExternalProxyIngressBackend struct {
	// port of the referenced service. A port name or port number
	// is required for a ExternalProxyServiceBackendPort.
	Port ExternalProxyServiceBackendPort `json:"port,omitempty"`
}

// ExternalProxyServiceBackendPort is the service port being referenced.
type ExternalProxyServiceBackendPort struct {
	// name is the name of the port on the Service.
	// This is a mutually exclusive setting with "Number".
	// +optional
	Name string `json:"name,omitempty"`

	// number is the numerical port number (e.g. 80) on the Service.
	// This is a mutually exclusive setting with "Name".
	// +optional
	Number int32 `json:"number,omitempty"`
}

// ExternalProxyStatus defines the observed state of ExternalProxy
type ExternalProxyStatus struct {
	Ready              bool   `json:"ready"`
	ServiceName        string `json:"serviceName"`
	ObservedGeneration int64  `json:"observedGeneration"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=".status.serviceName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status

// ExternalProxy is the Schema for the externalproxies API
type ExternalProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalProxySpec   `json:"spec,omitempty"`
	Status ExternalProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalProxyList contains a list of ExternalProxy
type ExternalProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalProxy{}, &ExternalProxyList{})
}
