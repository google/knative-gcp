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

package ingress

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

const (
	testPath = "/namespace/broker"
	testNS   = "namespace"
	testName = "broker"
)

func TestBrokerPath(t *testing.T) {
	want := testPath
	got := BrokerPath(testNS, testName)
	if got != want {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestConvertPathtoNamespacedName(t *testing.T) {
	want := types.NamespacedName{
		Namespace: testNS,
		Name:      testName,
	}
	got, err := ConvertPathToNamespacedName(testPath)
	if err != nil {
		t.Errorf("unexpected error parsing broker path: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("unexpected conversion result: want %v, got %v", want, got)
	}
}
