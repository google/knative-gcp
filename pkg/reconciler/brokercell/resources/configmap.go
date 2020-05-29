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
