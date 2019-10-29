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

	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis/v1alpha1"

	"knative.dev/pkg/apis"
)

func (d *Decorator) Validate(ctx context.Context) *apis.FieldError {
	return d.Spec.Validate(ctx).ViaField("spec")
}

func (ds *DecoratorSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Sink [required]
	if equality.Semantic.DeepEqual(ds.Sink, v1alpha1.Destination{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := ds.Sink.Validate(ctx); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}

	return errs
}
