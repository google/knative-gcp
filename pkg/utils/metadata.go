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

package utils

import (
	"context"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/logging"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

const (
	clusterNameAttr = "cluster-name"
	ProjectIDEnvKey = "PROJECT_ID"
)

// defaultMetadataClientCreator is a create function to get a default metadata client. This can be
// swapped during testing.
var defaultMetadataClientCreator func() metadataClient.Client = metadataClient.NewDefaultMetadataClient

// ProjectIDEnvConfig is a struct to parse project ID from env var
type ProjectIDEnvConfig struct {
	ProjectID string `envconfig:"PROJECT_ID"`
}

// ProjectID returns the project ID for a particular resource.
func ProjectID(project string, client metadataClient.Client) (string, error) {
	// If project is set, then return that one.
	if project != "" {
		return project, nil
	}
	// Otherwise, ask GKE metadata server.
	projectID, err := client.ProjectID()
	if err != nil {
		return "", err
	}
	return projectID, nil
}

// ProjectIDOrDefault returns the project ID by performing the following order:
// 1) if the input project ID is valid, simply use it.
// 2) if there is a PROJECT_ID environmental variable, use it.
// 3) use metadataClient to resolve project id.
func ProjectIDOrDefault(ctx context.Context, projectID string) (string, error) {
	if projectID != "" {
		return projectID, nil
	}
	// Parse project id from env var, if not found we simply continue
	var env ProjectIDEnvConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Error("Failed to process env var", zap.Error(err))
	}
	return ProjectID(env.ProjectID, defaultMetadataClientCreator())
}

// ClusterName returns the cluster name for a particular resource.
func ClusterName(clusterName string, client metadataClient.Client) (string, error) {
	// If clusterName is set, then return that one.
	if clusterName != "" {
		return clusterName, nil
	}
	clusterName, err := client.InstanceAttributeValue(clusterNameAttr)
	if err != nil {
		return "", err
	}
	return clusterName, nil
}
