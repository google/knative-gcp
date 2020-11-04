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
	"fmt"

	"github.com/google/knative-gcp/pkg/testing/testloggingutil"

	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// PublisherArgs are the arguments needed to create a Topic publisher.
// Every field is required.
type PublisherArgs struct {
	Image  string
	Topic  *v1.Topic
	Labels map[string]string

	TracingConfig string
}

const (
	credsVolume    = "google-cloud-key"
	credsMountPath = "/var/secrets/google"
)

// DefaultSecretSelector is the default secret selector used to load the creds
// for the publisher to auth with Google Cloud.
func DefaultSecretSelector() *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "google-cloud-key",
		},
		Key: "key.json",
	}
}

func makePublisherPodSpec(args *PublisherArgs) *corev1.PodSpec {
	publisherContainer := corev1.Container{
		Image: args.Image,
		Env: []corev1.EnvVar{{
			Name:  "PROJECT_ID",
			Value: args.Topic.Spec.Project,
		}, {
			Name:  "PUBSUB_TOPIC_ID",
			Value: args.Topic.Spec.Topic,
		}, {
			Name:  "K_TRACING_CONFIG",
			Value: args.TracingConfig,
		}},
	}

	// This is added purely for the TestCloudLogging E2E tests, which verify that the log line is
	// written certain annotations are present.
	publisherContainer.Env = testloggingutil.PropagateLoggingE2ETestAnnotation(
		args.Topic.Annotations, publisherContainer.Env)

	// If k8s service account is specified, use that service account as credential.
	if args.Topic.Spec.ServiceAccountName != "" {
		return &corev1.PodSpec{
			ServiceAccountName: args.Topic.Spec.ServiceAccountName,
			Containers: []corev1.Container{
				publisherContainer,
			},
		}
	}

	// Otherwise, use secret as credential.
	secret := args.Topic.Spec.Secret
	if secret == nil {
		secret = DefaultSecretSelector()
	}
	credsFile := fmt.Sprintf("%s/%s", credsMountPath, secret.Key)

	publisherContainer.Env = append(publisherContainer.Env, corev1.EnvVar{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: credsFile,
	})
	publisherContainer.VolumeMounts = []corev1.VolumeMount{{
		Name:      credsVolume,
		MountPath: credsMountPath,
	}}

	return &corev1.PodSpec{
		Containers: []corev1.Container{
			publisherContainer,
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

// MakePublisher generates (but does not insert into K8s) the Invoker Deployment for
// Channels.
func MakePublisher(args *PublisherArgs) *servingv1.Service {
	podSpec := makePublisherPodSpec(args)

	return &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Topic.Namespace,
			Name:            GeneratePublisherName(args.Topic),
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Topic)},
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: args.Labels,
					},
					Spec: servingv1.RevisionSpec{
						PodSpec: *podSpec,
					},
				},
			},
		},
	}
}
