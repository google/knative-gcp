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

	"knative.dev/pkg/ptr"
)

const (
	minReplicas int32 = 1
	maxReplicas int32 = 10
)

// SetDefaults sets the default field values for a BrokerCell.
func (bc *BrokerCell) SetDefaults(ctx context.Context) {
	// Set defaults for the Spec.Components values.
	bc.Spec.SetDefaults(ctx)
}

// SetDefaults sets the default field values for a BrokerCellSpec.
func (bcs *BrokerCellSpec) SetDefaults(ctx context.Context) {
	if bcs.Components.Fanout.MinReplicas == nil {
		bcs.Components.Fanout.MinReplicas = ptr.Int32(minReplicas)
	}
	if bcs.Components.Fanout.MaxReplicas == nil {
		bcs.Components.Fanout.MaxReplicas = ptr.Int32(maxReplicas)
	}
	if bcs.Components.Retry.MinReplicas == nil {
		bcs.Components.Retry.MinReplicas = ptr.Int32(minReplicas)
	}
	if bcs.Components.Retry.MaxReplicas == nil {
		bcs.Components.Retry.MaxReplicas = ptr.Int32(maxReplicas)
	}
	if bcs.Components.Ingress.MinReplicas == nil {
		bcs.Components.Ingress.MinReplicas = ptr.Int32(minReplicas)
	}
	if bcs.Components.Ingress.MaxReplicas == nil {
		bcs.Components.Ingress.MaxReplicas = ptr.Int32(maxReplicas)
	}
}
