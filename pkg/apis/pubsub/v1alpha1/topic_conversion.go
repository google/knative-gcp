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

package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1beta1.PullSubscription) into v1alpha1.PullSubscription.
func (source *Topic) ConvertTo(_ context.Context, to apis.Convertible) error {
	/*
		switch sink := to.(type) {
		case *v1beta1.Topic:
			// The conversions are lossless, so we can just use the other direction here. If the
			// conversion eventually becomes lossy, then we won't be able to do this.
			return sink.ConvertFrom(nil, source)
		default:
			return fmt.Errorf("unknown conversion, got: %T", sink)

		}
	*/
	return nil
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1alpha1.PullSubscription into v1beta1.PullSubscription.
func (sink *Topic) ConvertFrom(_ context.Context, from apis.Convertible) error {
	/*
		switch source := from.(type) {
		case *v1beta1.Topic:
			sink.ObjectMeta = source.ObjectMeta
			return nil
		default:
			return fmt.Errorf("unknown conversion, got: %T", source)
		}
	*/
	return nil
}
