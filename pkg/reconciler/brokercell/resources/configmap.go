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

package resources

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"knative.dev/pkg/kmeta"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/broker/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	targetsCMName = "broker-targets"
	targetsCMKey  = "targets"
)

// TargetsConfigMapEqual compares the binary data contained in two TargetsConfig
// ConfigMaps and returns true if and only if the inputs are valid and the
// unmarshaled binary data are equal.
func TargetsConfigMapEqual(cm1, cm2 *corev1.ConfigMap) bool {
	// The broker targets ConfigMap BinaryData holds the serialized TargetsConfig
	// proto, and therefore cannot be safely compared with equality.Semantic.DeepEqual.
	// Instead, use proto.Equal to compare protos.
	v1, ok := cm1.BinaryData[targetsCMKey]
	if !ok {
		return false
	}
	v2, ok := cm2.BinaryData[targetsCMKey]
	if !ok {
		return false
	}
	proto1 := &config.TargetsConfig{}
	proto2 := &config.TargetsConfig{}
	if err := proto.Unmarshal(v1, proto1); err != nil {
		return false
	}
	if err := proto.Unmarshal(v2, proto2); err != nil {
		return false
	}
	return proto.Equal(proto1, proto2)
}

func MakeTargetsConfig(bc *intv1alpha1.BrokerCell, brokerTargets config.Targets) (*corev1.ConfigMap, error) {
	data, err := brokerTargets.Bytes()
	if err != nil {
		return nil, fmt.Errorf("error serializing targets config: %w", err)
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            Name(bc.Name, targetsCMName),
			Namespace:       bc.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(bc)},
			Labels:          Labels(bc.Name, "broker-targets"),
		},
		BinaryData: map[string][]byte{targetsCMKey: data},
		// Write out the text version for debugging purposes only
		Data: map[string]string{"targets.txt": brokerTargets.String()},
	}, nil
}
