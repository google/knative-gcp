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

	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/identity/resources"
	"github.com/google/knative-gcp/pkg/utils"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReceiveAdapterArgs are the arguments needed to create a PullSubscription Receive
// Adapter. Every field is required.
type ReceiveAdapterArgs struct {
	Image          string
	Source         *v1alpha1.PullSubscription
	Labels         map[string]string
	SubscriptionID string
	SinkURI        *apis.URL
	TransformerURI *apis.URL
	MetricsConfig  string
	LoggingConfig  string
	TracingConfig  string
}

const (
	credsVolume          = "google-cloud-key"
	credsMountPath       = "/var/secrets/google"
	metricsDomain        = "cloud.google.com/events"
	defaultResourceGroup = "pullsubscriptions.pubsub.cloud.google.com"
)

func makeReceiveAdapterPodSpec(ctx context.Context, args *ReceiveAdapterArgs) *corev1.PodSpec {
	// Convert CloudEvent Overrides to pod embeddable properties.
	ceExtensions := ""
	if args.Source.Spec.CloudEventOverrides != nil && args.Source.Spec.CloudEventOverrides.Extensions != nil {
		var err error
		ceExtensions, err = utils.MapToBase64(args.Source.Spec.CloudEventOverrides.Extensions)
		if err != nil {
			logging.FromContext(ctx).Warnw("failed to make cloudevents overrides extensions",
				zap.Error(err),
				zap.Any("extensions", args.Source.Spec.CloudEventOverrides.Extensions))
		}
	}

	var mode converters.ModeType
	switch args.Source.PubSubMode() {
	case "", v1alpha1.ModeCloudEventsBinary:
		mode = converters.Binary
	case v1alpha1.ModeCloudEventsStructured:
		mode = converters.Structured
	case v1alpha1.ModePushCompatible:
		mode = converters.Push
	}

	var resourceGroup = defaultResourceGroup
	if rg, ok := args.Source.Annotations["metrics-resource-group"]; ok {
		resourceGroup = rg
	}
	// Needed for Channels, as we use a generate name for the PullSubscription.
	var resourceName = args.Source.Name
	if rn, ok := args.Source.Annotations["metrics-resource-name"]; ok {
		resourceName = rn
	}

	var transformerURI string
	if args.TransformerURI != nil {
		transformerURI = args.TransformerURI.String()
	}

	receiveAdapterContainer := corev1.Container{
		Name:  "receive-adapter",
		Image: args.Image,
		Env: []corev1.EnvVar{{
			Name:  "PROJECT_ID",
			Value: args.Source.Spec.Project,
		}, {
			Name:  "PUBSUB_TOPIC_ID",
			Value: args.Source.Spec.Topic,
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
			Value: args.Source.Spec.AdapterType,
		}, {
			Name:  "SEND_MODE",
			Value: string(mode),
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
			Value: args.Source.Namespace,
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

	// If GCP service account is specified, use that service account as credential.
	if args.Source.Spec.GoogleServiceAccount != "" {
		kServiceAccountName := resources.GenerateServiceAccountName(args.Source.Spec.GoogleServiceAccount)
		return &corev1.PodSpec{
			ServiceAccountName: kServiceAccountName,
			Containers: []corev1.Container{
				receiveAdapterContainer,
			},
		}
	}

	// Otherwise, use secret as credential.
	secret := args.Source.Spec.Secret
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
			Namespace:       args.Source.Namespace,
			Name:            GenerateSubscriptionName(args.Source),
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Source)},
			// Copy the source annotations so that the appropriate reconciler is called.
			Annotations: args.Source.Annotations,
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
