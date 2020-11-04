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

package resources

import (
	"context"
	"fmt"

	"github.com/google/knative-gcp/pkg/testing/testloggingutil"

	"go.uber.org/zap"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/intevents"
	intereventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/utils"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReceiveAdapterArgs are the arguments needed to create a PullSubscription Receive
// Adapter. Every field is required.
type ReceiveAdapterArgs struct {
	Image            string
	PullSubscription *intereventsv1.PullSubscription
	Labels           map[string]string
	SubscriptionID   string
	SinkURI          *apis.URL
	TransformerURI   *apis.URL
	MetricsConfig    string
	LoggingConfig    string
	TracingConfig    string
}

const (
	credsVolume          = "google-cloud-key"
	credsMountPath       = "/var/secrets/google"
	metricsDomain        = "cloud.google.com/events"
	defaultResourceGroup = "pullsubscriptions.internal.events.cloud.google.com"
)

func makeReceiveAdapterPodSpec(ctx context.Context, args *ReceiveAdapterArgs) *corev1.PodSpec {
	// Convert CloudEvent Overrides to pod embeddable properties.
	ceExtensions := ""
	if args.PullSubscription.Spec.CloudEventOverrides != nil && args.PullSubscription.Spec.CloudEventOverrides.Extensions != nil {
		var err error
		ceExtensions, err = utils.MapToBase64(args.PullSubscription.Spec.CloudEventOverrides.Extensions)
		if err != nil {
			logging.FromContext(ctx).Warnw("failed to make cloudevents overrides extensions",
				zap.Error(err),
				zap.Any("extensions", args.PullSubscription.Spec.CloudEventOverrides.Extensions))
		}
	}

	var resourceGroup = defaultResourceGroup
	if rg, ok := args.PullSubscription.Annotations["metrics-resource-group"]; ok {
		resourceGroup = rg
	}
	// Needed for Channels, as we use a generate name for the PullSubscription.
	var resourceName = args.PullSubscription.Name
	if rn, ok := args.PullSubscription.Annotations["metrics-resource-name"]; ok {
		resourceName = rn
	}

	var transformerURI string
	if args.TransformerURI != nil {
		transformerURI = args.TransformerURI.String()
	}

	adapterType := args.PullSubscription.Spec.AdapterType
	// If the PullSubscription has no Channel nor Source label, means that users created a PullSubscription manually.
	// Then we set the adapter type to be PubSubPull.
	_, isFromSource := args.PullSubscription.Labels[intevents.SourceLabelKey]
	_, isFromChannel := args.PullSubscription.Labels[intevents.ChannelLabelKey]
	if !isFromSource && !isFromChannel {
		adapterType = string(converters.PubSubPull)
	}

	receiveAdapterContainer := corev1.Container{
		Name:  "receive-adapter",
		Image: args.Image,
		// Such resources setting should support tps with 1000/s
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				// The memory limit we set is 600Mi which is mostly used to prevent surging memory usage causing OOM.
				corev1.ResourceMemory: resource.MustParse("600Mi"),
				corev1.ResourceCPU:    resource.MustParse("500m"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("50Mi"),
				corev1.ResourceCPU:    resource.MustParse("400m"),
			},
		},
		Env: []corev1.EnvVar{{
			Name:  "PROJECT_ID",
			Value: args.PullSubscription.Spec.Project,
		}, {
			Name:  "PUBSUB_TOPIC_ID",
			Value: args.PullSubscription.Spec.Topic,
		}, {
			Name:  "PUBSUB_SUBSCRIPTION_ID",
			Value: args.SubscriptionID,
		}, {
			Name:  "SINK_URI",
			Value: args.SinkURI.String(),
		}, {
			Name:  "TRANSFORMER_URI",
			Value: transformerURI,
		}, {
			Name:  "ADAPTER_TYPE",
			Value: adapterType,
		}, {
			Name:  "K_CE_EXTENSIONS",
			Value: ceExtensions,
		}, {
			Name:  "K_METRICS_CONFIG",
			Value: args.MetricsConfig,
		}, {
			Name:  "K_LOGGING_CONFIG",
			Value: args.LoggingConfig,
		}, {
			Name:  "K_TRACING_CONFIG",
			Value: args.TracingConfig,
		}, {
			Name:  "NAME",
			Value: resourceName,
		}, {
			Name:  "NAMESPACE",
			Value: args.PullSubscription.Namespace,
		}, {
			Name:  "RESOURCE_GROUP",
			Value: resourceGroup,
		}, {
			Name:  "METRICS_DOMAIN",
			Value: metricsDomain,
		}},
		Ports: []corev1.ContainerPort{{
			Name:          "metrics",
			ContainerPort: 9090,
		}},
	}

	// This is added purely for the TestCloudLogging E2E tests, which verify that the log line is
	// written certain annotations are present.
	receiveAdapterContainer.Env = testloggingutil.PropagateLoggingE2ETestAnnotation(
		args.PullSubscription.Annotations, receiveAdapterContainer.Env)

	// If there is no secret to embed, return what we have.
	if args.PullSubscription.Spec.Secret == nil {
		return &corev1.PodSpec{
			ServiceAccountName: args.PullSubscription.Spec.ServiceAccountName,
			Containers: []corev1.Container{
				receiveAdapterContainer,
			},
		}
	}

	// Otherwise, use secret as credential.
	secret := args.PullSubscription.Spec.Secret
	credsFile := fmt.Sprintf("%s/%s", credsMountPath, secret.Key)

	receiveAdapterContainer.Env = append(
		receiveAdapterContainer.Env,
		corev1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: credsFile,
		},
		corev1.EnvVar{
			// Needed for Keda scaling.
			// TODO set it only when using Keda.
			Name:      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: secret},
		})

	receiveAdapterContainer.VolumeMounts = []corev1.VolumeMount{{
		Name:      credsVolume,
		MountPath: credsMountPath,
	}}

	return &corev1.PodSpec{
		ServiceAccountName: args.PullSubscription.Spec.ServiceAccountName,
		Containers: []corev1.Container{
			receiveAdapterContainer,
		},
		Volumes: []corev1.Volume{{
			Name: credsVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		}},
	}
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// PullSubscriptions.
func MakeReceiveAdapter(ctx context.Context, args *ReceiveAdapterArgs) *v1.Deployment {
	podSpec := makeReceiveAdapterPodSpec(ctx, args)
	replicas := int32(1)

	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.PullSubscription.Namespace,
			Name:            GenerateReceiveAdapterName(args.PullSubscription),
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.PullSubscription)},
			// Copy the source annotations so that the appropriate reconciler is called.
			Annotations: args.PullSubscription.Annotations,
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: args.Labels,
				},
				Spec: *podSpec,
			},
		},
	}
}
