/*
Copyright 2019 Google LLC.

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

package operations

import (
	"testing"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	secretName = "google-cloud-key"
)

func TestValidateOpCtx(t *testing.T) {
	tests := []struct {
		name        string
		f           func(OpCtx) OpCtx
		expectedErr string
	}{{
		name: "missing UID",
		f: func(o OpCtx) OpCtx {
			o.UID = ""
			return o
		},
		expectedErr: "missing UID",
	}, {
		name: "missing Image",
		f: func(o OpCtx) OpCtx {
			o.Image = ""
			return o
		},
		expectedErr: "missing Image",
	}, {
		name: "missing secret",
		f: func(o OpCtx) OpCtx {
			o.Secret = corev1.SecretKeySelector{}
			return o
		},
		expectedErr: "invalid secret missing name or key",
	}, {
		name: "missing owner",
		f: func(o OpCtx) OpCtx {
			o.Owner = nil
			return o
		},
		expectedErr: "missing owner",
	}, {
		name:        "valid OpCtx",
		f:           func(o OpCtx) OpCtx { return o },
		expectedErr: "",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateOpCtx(test.f(OpCtx{
				UID:   "uid",
				Image: "image",
				Secret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: "key.json",
				},
				Owner: &v1alpha1.Topic{},
			}))
			if (err == nil && test.expectedErr != "") ||
				(err != nil && err.Error() != test.expectedErr) {
				t.Errorf("Error mismatch, want: %q got: %q", test.expectedErr, err)
			}
		})
	}
}
