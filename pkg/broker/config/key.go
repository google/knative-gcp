/*
Copyright 2020 Google LLC

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

package config

import (
	"fmt"
	"strings"

	"go.opencensus.io/trace"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"go.opencensus.io/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	kntracing "knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/metrics/metricskey"
)

// BrokerKey uniquely identifies a single Broker, at a given point in time.
type BrokerKey struct {
	namespace string
	name      string
}

// String creates a human readable version of this key. It is for debug purposes only. It is free to
// change at any time.
func (k *BrokerKey) String() string {
	// Note that this is explicitly different than the PersistenceString, so that we don't
	// accidentally use String(), rather than PersistenceString().
	return k.namespace + "//" + k.name
}

// PersistenceString is the string that is persisted as the key for this Broker in the protobuf. It
// is stable and can only change if all existing usage locations are made backwards compatible,
// supporting _both_ the old and the new format, for at least one release.
func (k *BrokerKey) PersistenceString() string {
	return k.namespace + "/" + k.name
}

func BrokerKeyFromPersistenceString(s string) (*BrokerKey, error) {
	pieces := strings.Split(s, "/")
	if len(pieces) != 3 {
		return nil, fmt.Errorf("malformed request path; expect format '/<ns>/<broker>', actually %q", s)
	}
	// Broker's persistence strings are in the form "/<ns>/<brokerName>".
	blank, ns, brokerName := pieces[0], pieces[1], pieces[2]
	if blank != "" {
		return nil, fmt.Errorf("malformed request path; expect format '/<ns>/<broker>', actually %q", s)
	}
	if err := validateNamespace(ns); err != nil {
		return nil, err
	}
	if err := validateBrokerName(brokerName); err != nil {
		return nil, err
	}
	return &BrokerKey{
		namespace: ns,
		name:      brokerName,
	}, nil
}

// MetricsResource generates the Resource object that metrics will be associated with.
func (k *BrokerKey) MetricsResource() resource.Resource {
	return resource.Resource{
		Type: metricskey.ResourceTypeKnativeBroker,
		Labels: map[string]string{
			metricskey.LabelNamespaceName: k.namespace,
			metricskey.LabelBrokerName:    k.name,
		},
	}
}

// CreateEmptyBroker creates an empty Broker that corresponds to this BrokerKey. It is empty except
// for the portions known about by the BrokerKey.
func (k *BrokerKey) CreateEmptyBroker() *CellTenant {
	return &CellTenant{
		Namespace: k.namespace,
		Name:      k.name,
	}
}

// SpanMessagingDestination is the Messaging Destination of requests sent to this BrokerKey.
func (k *BrokerKey) SpanMessagingDestination() string {
	return kntracing.BrokerMessagingDestination(k.namespacedName())
}

// SpanMessagingDestinationAttribute is the Messaging Destination attribute that should be attached
// to the tracing Span.
func (k *BrokerKey) SpanMessagingDestinationAttribute() trace.Attribute {
	return kntracing.BrokerMessagingDestinationAttribute(k.namespacedName())
}

func (k *BrokerKey) namespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: k.namespace,
		Name:      k.name,
	}
}

// Key returns the BrokerKey for this Broker.
func (x *CellTenant) Key() *BrokerKey {
	return &BrokerKey{
		namespace: x.Namespace,
		name:      x.Name,
	}
}

// TargetKey uniquely identifies a single Target, at a given point in time.
type TargetKey struct {
	brokerKey BrokerKey
	name      string
}

// ParentKey is the key of the parent this Target corresponds to.
func (k *TargetKey) ParentKey() *BrokerKey {
	return &k.brokerKey
}

// String creates a human readable version of this key. It is for debug purposes only. It is free to
// change at any time.
func (k *TargetKey) String() string {
	// Note that this is explicitly different than the PersistenceString, so that we don't
	// accidentally use String(), rather than PersistenceString().
	return k.brokerKey.String() + "//" + k.name
}

// Key returns the TargetKey for this Target.
func (x *Target) Key() *TargetKey {
	return &TargetKey{
		brokerKey: BrokerKey{
			namespace: x.Namespace,
			name:      x.CellTenantName,
		},
		name: x.Name,
	}
}

// KeyFromBroker creates a BrokerKey from a K8s Broker object.
func KeyFromBroker(b *brokerv1beta1.Broker) *BrokerKey {
	return &BrokerKey{
		namespace: b.Namespace,
		name:      b.Name,
	}
}

// TestOnlyBrokerKey returns the key of a broker. This method exists to make tests that need a
// BrokerKey, but do not need an actual Broker, easier to write.
func TestOnlyBrokerKey(namespace, name string) *BrokerKey {
	return &BrokerKey{
		namespace: namespace,
		name:      name,
	}
}

// validateNamespace validates that the given string is a valid K8s namespace.
func validateNamespace(ns string) error {
	errs := validation.IsDNS1123Label(ns)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid namespace %q, %v", ns, errs)
}

// validateBrokerName validates that the given string is a valid name for a Broker in K8s.
func validateBrokerName(name string) error {
	errs := validation.IsDNS1123Label(name)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid name %q, %v", name, errs)
}
