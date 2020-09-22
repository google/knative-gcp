package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/utils"
)

// NewDefaultPubsubClient creates a new pubsub client from specified project id or default
// gcp metadata, the early a project id is in the argument list, the higher priority it will
// have. Client is always set to nil in error condition.
func NewDefaultPubsubClient(ctx context.Context, projectIDs ...string) (*pubsub.Client, error) {
	projectID := ""
	for _, id := range projectIDs {
		if id != "" {
			projectID = id
			break
		}
	}
	projectID, err := utils.ProjectID(projectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		return nil, err
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return client, nil
}
