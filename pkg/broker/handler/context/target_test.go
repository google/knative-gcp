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

package context

import (
	"context"
	"testing"

	"github.com/google/knative-gcp/pkg/broker/config"
)

func TestTargetKey(t *testing.T) {
	_, err := GetTargetKey(context.Background())
	if err != ErrTargetKeyNotPresent {
		t.Errorf("error from GetTargetKey got=%v, want=%v", err, ErrTargetKeyNotPresent)
	}
	wantTarget := (&config.Target{
		Id:             "456",
		Name:           "my-target",
		Namespace:      "namespace",
		CellTenantType: config.CellTenantType_BROKER,
		CellTenantName: "broker-name",
		Address:        "baz",
	}).Key()
	ctx := WithTargetKey(context.Background(), wantTarget)
	gotTarget, err := GetTargetKey(ctx)
	if gotTarget != wantTarget {
		t.Errorf("GetTargetKey got=%v, want=%v", gotTarget, wantTarget)
	}
}
