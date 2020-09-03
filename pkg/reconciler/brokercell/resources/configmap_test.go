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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "knative.dev/pkg/system/testing"
)

func TestTargetsConfigMapEqual(t *testing.T) {
	var (
		targets1 = &config.TargetsConfig{
			Brokers: map[string]*config.Broker{
				"ns/broker1": {
					Name:      "broker1",
					Namespace: "ns",
				},
			},
		}
		targets1Bytes, _ = proto.Marshal(targets1)
		targets2         = &config.TargetsConfig{
			Brokers: map[string]*config.Broker{
				"ns/broker2": {
					Name:      "broker2",
					Namespace: "ns",
				},
			},
		}
		targets2Bytes, _ = proto.Marshal(targets2)

		targets1Cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: targetsCMName},
			BinaryData: map[string][]byte{targetsCMKey: targets1Bytes},
		}
		targets2Cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: targetsCMName},
			BinaryData: map[string][]byte{targetsCMKey: targets2Bytes},
		}
		invalidProtoCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: targetsCMName},
			BinaryData: map[string][]byte{targetsCMKey: {'b'}},
		}
		notTargetsCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: targetsCMName},
			BinaryData: map[string][]byte{"some-key": targets1Bytes},
		}
	)
	var tests = []struct {
		name   string
		cm1    *corev1.ConfigMap
		cm2    *corev1.ConfigMap
		wantEq bool
	}{
		{
			name:   "same targets config",
			cm1:    targets1Cm,
			cm2:    targets1Cm,
			wantEq: true,
		},
		{
			name:   "different targets config",
			cm1:    targets1Cm,
			cm2:    targets2Cm,
			wantEq: false,
		},
		{
			name:   "invalid proto bytes",
			cm1:    invalidProtoCm,
			cm2:    invalidProtoCm,
			wantEq: false,
		},
		{
			name:   "not targets config",
			cm1:    notTargetsCm,
			cm2:    notTargetsCm,
			wantEq: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if res := TargetsConfigMapEqual(test.cm1, test.cm2); res != test.wantEq {
				t.Errorf("Unexpected equality result %t between ConfigMaps, wanted %t; diff: %s", res, test.wantEq, cmp.Diff(test.cm1, test.cm2))
			}
		})
	}
}

func TestMakeTargetsConfig(t *testing.T) {
	_, err := MakeTargetsConfig(NewBrokerCell("name", "ns"), memory.NewEmptyTargets())
	if err != nil {
		t.Errorf("Error making TargetsConfig: %v", err)
	}
}
