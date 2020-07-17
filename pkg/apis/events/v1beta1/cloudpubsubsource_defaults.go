/*
Copyright 2019 Google LLC

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

package v1beta1

import (
	"context"

	"github.com/google/knative-gcp/pkg/apis/duck"
	"github.com/google/knative-gcp/pkg/apis/intevents"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

func (ps *CloudPubSubSource) SetDefaults(ctx context.Context) {
	ctx = apis.WithinParent(ctx, ps.ObjectMeta)
	ps.Spec.SetDefaults(ctx)
	duck.SetAutoscalingAnnotationsDefaults(ctx, &ps.ObjectMeta)
}

func (pss *CloudPubSubSourceSpec) SetDefaults(ctx context.Context) {
	pss.SetPubSubDefaults(ctx)

	if pss.AckDeadline == nil {
		ackDeadline := intevents.DefaultAckDeadline
		pss.AckDeadline = ptr.String(ackDeadline.String())
	}

	if pss.RetentionDuration == nil {
		retentionDuration := intevents.DefaultRetentionDuration
		pss.RetentionDuration = ptr.String(retentionDuration.String())
	}
}
