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
	fakeValue                   = "asdfasdfasdfasdf"
)

func AuthCheckForAuthTypeTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	t.Helper()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	// Initialize client in workload identity mode, no matter the real mode is workload identity or secret,
	// so that a default secret will not be created in this test namespace.
	client := lib.Setup(ctx, t, true, true)
	defer lib.TearDown(ctx, client)

	serviceAccountName := authConfig.ServiceAccountName
	if authConfig.WorkloadIdentity {
		serviceAccountName = authCheckServiceAccountName
	}

	pubSubConfig := lib.PubSubConfig{
		SinkGVK:            lib.ServiceGVK,
		PubSubName:         psName,
		SinkName:           svcName,
		TopicName:          topic,
		ServiceAccountName: serviceAccountName,
	}

	lib.MakePubSub(client, pubSubConfig)

	// Authentication check will return error for:
	// 1. In Workload Identity mode, there is no k8s service account.
	// 2. In Secret mode, there is no k8s secret.
	client.WaitForSourceAuthCheckPendingOrFail(psName, lib.CloudPubSubSourceV1TypeMeta)
}

func AuthCheckForPodCheckTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	t.Helper()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	// Initialize client in workload identity mode, no matter the real mode is workload identity or secret,
	// so that a default secret will not be created in this test namespace.
	client := lib.Setup(ctx, t, true, true)
	defer lib.TearDown(ctx, client)

	serviceAccountName := authConfig.ServiceAccountName
	if authConfig.WorkloadIdentity {
		serviceAccountName = authCheckServiceAccountName
		so := make([]reconcilertesting.ServiceAccountOption, 0)
		so = append(so, reconcilertesting.WithServiceAccountAnnotation(fakeServiceAccount))
		serviceAccount := reconcilertesting.NewServiceAccount(serviceAccountName, client.Namespace, so...)
		client.CreateServiceAccountOrFail(serviceAccount)
	} else {
		so := make([]reconcilertesting.SecretOption, 0)
		so = append(so, reconcilertesting.WithData(map[string][]byte{
			"key.json": []byte(fakeValue),
		}))
		secret := reconcilertesting.NewSecret("google-cloud-key", client.Namespace, so...)
		client.CreateSecretOrFail(secret)
	}

	pubSubConfig := lib.PubSubConfig{
		SinkGVK:            lib.ServiceGVK,
		PubSubName:         psName,
		SinkName:           svcName,
		TopicName:          topic,
		ServiceAccountName: serviceAccountName,
	}

	lib.MakePubSub(client, pubSubConfig)

	// Authentication check will return error for:
	// 1. In Workload Identity mode, relationship between ksa and gsa is uncompleted.
	// 2. In Secret mode, the key in k8s secret is a fake one.
	client.WaitForSourceAuthCheckPendingOrFail(psName, lib.CloudPubSubSourceV1TypeMeta)
}
