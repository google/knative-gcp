/*
Copyright 2020 Google LLC.

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

package v1

import (
	"testing"

	v1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/pkg/apis/duck"
)

func TestTypesImplements(t *testing.T) {
	testCases := []struct {
		instance interface{}
		iface    duck.Implementable
	}{
		{instance: &CloudAuditLogsSource{}, iface: &v1.Source{}},
		{instance: &CloudAuditLogsSource{}, iface: &v1.Conditions{}},
		{instance: &CloudStorageSource{}, iface: &v1.Source{}},
		{instance: &CloudStorageSource{}, iface: &v1.Conditions{}},
		{instance: &CloudSchedulerSource{}, iface: &v1.Source{}},
		{instance: &CloudSchedulerSource{}, iface: &v1.Conditions{}},
		{instance: &CloudPubSubSource{}, iface: &v1.Source{}},
		{instance: &CloudPubSubSource{}, iface: &v1.Conditions{}},
		{instance: &CloudBuildSource{}, iface: &v1.Source{}},
		{instance: &CloudBuildSource{}, iface: &v1.Conditions{}},
	}
	for _, tc := range testCases {
		if err := duck.VerifyType(tc.instance, tc.iface); err != nil {
			t.Error(err)
		}
	}
}
