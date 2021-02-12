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

package testing

import (
	"fmt"
	"strings"

	"github.com/google/knative-gcp/pkg/apis/configs/brokerdelivery"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/system"
)

// ConfigMapOption enables further configuration of a ConfigMap.
type ConfigMapOption func(*corev1.ConfigMap)

// NewConfigMap creates a ConfigMap with ConfigMapOptions.
func NewConfigMap(name, namespace string, cmo ...ConfigMapOption) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range cmo {
		opt(cm)
	}
	return cm
}

func WithConfigMapData(data map[string]string) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		cm.Data = data
	}
}

func WithConfigMapBinaryData(data map[string][]byte) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		cm.BinaryData = data
	}
}

func WithConfigMapDataEntry(key, value string) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[key] = value
	}
}

func WithConfigMapBinaryDataEntry(key string, value []byte) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		if cm.BinaryData == nil {
			cm.BinaryData = make(map[string][]byte)
		}
		cm.BinaryData[key] = value
	}
}

// NewDataresidencyConfigMapFromRegions Create new data residency configuration map
// from list of allowed persistence regions
func NewDataresidencyConfigMapFromRegions(regions []string) *corev1.ConfigMap {
	// Note that the data is in yaml, so no tab is allowed, use spaces instead.
	var sb strings.Builder
	sb.WriteString("\n  clusterDefaults:")
	sb.WriteString("\n    messagestoragepolicy.allowedpersistenceregions:")
	if regions == nil || len(regions) == 0 {
		sb.WriteString(" []")
	} else {
		for _, region := range regions {
			sb.WriteString("\n    - ")
			sb.WriteString(region)
		}
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataresidency.ConfigMapName(),
			Namespace: system.Namespace(),
		},
		Data: map[string]string{
			"default-dataresidency-config": sb.String(),
		},
	}
}

// NewBrokerDeliveryConfigMapFromDeliverySpec creates a new cluster defaulted
// broker delivery configuration map from a given delivery spec.
func NewBrokerDeliveryConfigMapFromDeliverySpec(spec *eventingduckv1beta1.DeliverySpec) *corev1.ConfigMap {
	var sb strings.Builder
	sb.WriteString("\n  clusterDefaults:")
	if spec != nil {
		if spec.BackoffPolicy != nil {
			sb.WriteString("\n    backoffPolicy: ")
			sb.WriteString(string(*spec.BackoffPolicy))
		}
		if spec.BackoffDelay != nil {
			sb.WriteString("\n    backoffDelay: ")
			sb.WriteString(*spec.BackoffDelay)
		}
		if spec.Retry != nil {
			sb.WriteString("\n    retry: ")
			sb.WriteString(fmt.Sprint(*spec.Retry))
		}
		if spec.DeadLetterSink != nil {
			sb.WriteString("\n    deadLetterSink: ")
			sb.WriteString("\n      uri: ")
			sb.WriteString(spec.DeadLetterSink.URI.String())
		}
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      brokerdelivery.ConfigMapName(),
			Namespace: system.Namespace(),
		},
		Data: map[string]string{
			"default-br-delivery-config": sb.String(),
		},
	}
}
