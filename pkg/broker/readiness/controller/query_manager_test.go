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

package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	messagingv1beta1 "github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	. "github.com/google/knative-gcp/pkg/broker/readiness"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testNS      = "testnamespace"
	brokerName  = "test-broker"
	triggerName = "test-trigger"
	channelName = "chan"
)

func TestGetGenerationForObject(t *testing.T) {
	testcases := []struct {
		name      string
		obj       runtime.Object
		want      int64
		wantError error
	}{
		{
			"getGenerationForObject with broker",
			brokerv1beta1.NewBroker(brokerName, testNS,
				brokerv1beta1.WithBrokerGeneration(10)),
			10,
			nil,
		},
		{
			"getGenerationForObject with channel",
			messagingv1beta1.NewChannel(channelName, testNS,
				messagingv1beta1.WithChannelGeneration(11)),
			11,
			nil,
		},
		{
			"getGenerationForObject with trigger",
			brokerv1beta1.NewTrigger(triggerName, testNS,
				brokerv1beta1.WithTriggerGeneration(12)),
			12,
			nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := getGenerationForObject(tc.obj)
			if diff := cmp.Diff(tc.wantError, err != nil); diff != "" {
				t.Error("unexpected error (-want, +got) = ", diff)
			}
		})
	}
}
