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

package operations

import (
	"github.com/knative/pkg/kmeta"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SubArgs struct {
	Image               string
	Action              string
	ProjectID           string
	TopicID             string
	SubscriptionID      string
	AckDeadline         time.Duration
	RetainAckedMessages bool
	RetentionDuration   time.Duration
	Owner               kmeta.OwnerRefable
}

func NewSubscriptionOps(arg SubArgs) *batchv1.Job {
	env := []corev1.EnvVar{{
		Name:  "ACTION",
		Value: arg.Action,
	}, {
		Name:  "PROJECT_ID",
		Value: arg.ProjectID,
	}, {
		Name:  "PUBSUB_TOPIC_ID",
		Value: arg.TopicID,
	}, {
		Name:  "PUBSUB_SUBSCRIPTION_ID",
		Value: arg.SubscriptionID,
	}}

	switch arg.Action {
	case ActionCreate:
		env = append(env, []corev1.EnvVar{{
			Name:  "PUBSUB_SUBSCRIPTION_CONFIG_ACK_DEAD",
			Value: arg.AckDeadline.String(),
		}, {
			Name:  "PUBSUB_SUBSCRIPTION_CONFIG_RET_ACKED",
			Value: strconv.FormatBool(arg.RetainAckedMessages),
		}, {
			Name:  "PUBSUB_SUBSCRIPTION_CONFIG_RET_DUR",
			Value: arg.RetentionDuration.String(),
		}}...)
	}

	podTemplate := makePodTemplate(arg.Image, env...)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    SubscriptionJobName(arg.Owner, arg.Action),
			Namespace:       arg.Owner.GetObjectMeta().GetNamespace(),
			Labels:          SubscriptionJobLabels(arg.Owner, arg.Action),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(arg.Owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}
}
