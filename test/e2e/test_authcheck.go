/*
Copyright 2021 Google LLC

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

package e2e

import (
	"context"
	"testing"

	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/lib"
)

const (
	authCheckServiceAccountName = "auth-check-test"
	fakeServiceAccount          = "fakeserviceaccount@test-project.iam.gserviceaccount.com"
	invalidSecretValue          = "{\n  \"type\": \"service_account\",\n  \"project_id\": \"fake-project-id\",\n  \"private_key_id\": \"fake-key-id\"}"
)

func AuthCheckForPodCheckTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	// Initialize client in workload identity mode, no matter the real mode is workload identity or secret,
	// so that a default secret will not be created in this test namespace.
	client := lib.Setup(ctx, t, true, true)
	defer lib.TearDown(ctx, client)

	serviceAccountName := authConfig.ServiceAccountName
	var wantMessage string
	if authConfig.WorkloadIdentity {
		serviceAccountName = authCheckServiceAccountName
		so := make([]reconcilertesting.ServiceAccountOption, 0)
		so = append(so, reconcilertesting.WithServiceAccountAnnotation(fakeServiceAccount))
		serviceAccount := reconcilertesting.NewServiceAccount(serviceAccountName, client.Namespace, so...)
		client.CreateServiceAccountOrFail(serviceAccount)
		wantMessage = "the Pod is not fully authenticated, probably due to corresponding k8s service account and google service account do not establish a correct relationship"
	} else {
		so := make([]reconcilertesting.SecretOption, 0)
		so = append(so, reconcilertesting.WithData(map[string][]byte{
			"key.json": []byte(invalidSecretValue),
		}))
		secret := reconcilertesting.NewSecret("google-cloud-key", client.Namespace, so...)
		client.CreateSecretOrFail(secret)
		wantMessage = "error getting the token, probably due to the key stored in the Kubernetes Secret is expired or revoked"
	}

	pubSubConfig := lib.PubSubConfig{
		SinkGVK:            lib.ServiceGVK,
		PubSubName:         psName,
		SinkName:           svcName,
		TopicName:          topic,
		ServiceAccountName: serviceAccountName,
	}

	lib.MakePubSub(client, pubSubConfig)

	// In this test, authentication check will mark Source with AuthenticationCheckPending Reason for:
	// 1. In Workload Identity mode, relationship between ksa and gsa is uncompleted.
	// 2. In Secret mode, the key in k8s secret is a invalid one.
	// Test will fail if authentication check does not mark Source with AuthenticationCheckPending Reason
	// and with Message includes the wanted message.
	client.WaitForSourceAuthCheckPendingOrFail(psName, lib.CloudPubSubSourceV1TypeMeta, wantMessage)
}

func AuthCheckForNonPodCheckTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	// Initialize client in workload identity mode, no matter the real mode is workload identity or secret,
	// so that a default secret will not be created in this test namespace.
	client := lib.Setup(ctx, t, true, true)
	defer lib.TearDown(ctx, client)

	serviceAccountName := authConfig.ServiceAccountName
	var wantMessage string
	if authConfig.WorkloadIdentity {
		serviceAccountName = authCheckServiceAccountName
		wantMessage = "can't find Kubernetes Service Account"
	} else {
		wantMessage = `secret "google-cloud-key" not found`
	}

	pubSubConfig := lib.PubSubConfig{
		SinkGVK:            lib.ServiceGVK,
		PubSubName:         psName,
		SinkName:           svcName,
		TopicName:          topic,
		ServiceAccountName: serviceAccountName,
	}

	lib.MakePubSub(client, pubSubConfig)

	// In this test, authentication check will mark Source with AuthenticationCheckPending Reason for:
	// 1. In Workload Identity mode, there is no k8s service account.
	// 2. In Secret mode, there is no k8s secret.
	// Test will fail if authentication check does not mark Source with AuthenticationCheckPending Reason
	// and with Message includes the wanted message.
	client.WaitForSourceAuthCheckPendingOrFail(psName, lib.CloudPubSubSourceV1TypeMeta, wantMessage)
}
