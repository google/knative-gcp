/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	"context"
	"testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/webhook/resourcesemantics"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestDecoratorValidation(t *testing.T) {
	tests := []struct {
		name string
		cr   resourcesemantics.GenericCRD
		want *apis.FieldError
	}{{
		name: "empty",
		cr: &Decorator{
			Spec: DecoratorSpec{},
		},
		want: &apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"spec.sink"},
		},
	}, {
		name: "sink missing kind",
		cr: &Decorator{
			Spec: DecoratorSpec{
				SourceSpec: v1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &corev1.ObjectReference{
							APIVersion: "v1awesome1",
							Name:       "aName",
						},
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"spec.sink.ref.kind"},
		},
	}, {
		name: "valid",
		cr: &Decorator{
			Spec: DecoratorSpec{
				SourceSpec: v1.SourceSpec{
					CloudEventOverrides: &v1.CloudEventOverrides{
						Extensions: map[string]string{
							"foo": "bar",
						},
					},
					Sink: duckv1.Destination{
						Ref: &corev1.ObjectReference{
							Kind:       "aKind",
							APIVersion: "v1awesome1",
							Name:       "aName",
						},
					},
				},
			},
		},
		want: nil,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cr.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: validate (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
