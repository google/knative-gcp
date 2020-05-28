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

package lib

import (
	"context"
	"os"
	"testing"

	"cloud.google.com/go/pubsub"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)


func SubscriptionExists(t *testing.T, subId string) bool {
	t.Helper()
	ctx := context.Background()
	// Prow sticks the project in this key
	project := os.Getenv(ProwProjectKey)
	if project == "" {
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
	}
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		t.Fatalf("failed to create pubsub client, %s", err.Error())
	}
	sub:=client.Subscription(subId)
	exists, err := sub.Exists(ctx)
	if err != nil {
		t.Fatalf("failed to verify whether Pub/Sub subscription exists, %s", err.Error())
	}
	return exists
}



