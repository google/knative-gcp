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
	"context"
	"fmt"

	"cloud.google.com/go/compute/metadata"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/knative-gcp/pkg/pubsub"
)

type PubSubOps struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// Client is the Pub/Sub client used by Ops.
	Client pubsub.Client
}

func (s *PubSubOps) CreateClient(ctx context.Context) error {
	if s.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			return err
		}
		s.Project = project
	}

	client, err := pubsub.NewClient(ctx, s.Project)
	if err != nil {
		return fmt.Errorf("failed to create Pub/Sub client: %s", err)
	}
	s.Client = client
	return nil
}

type PubSubArgs struct {
	ProjectID string
}

func PubSubEnv(args PubSubArgs) []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  "PROJECT_ID",
		Value: args.ProjectID,
	}}
}

func (a PubSubArgs) OperationGroup() string {
	return "pubsub"
}
