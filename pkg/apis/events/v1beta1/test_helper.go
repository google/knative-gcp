package v1beta1

import (
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/events"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// These variables are used to create a 'complete' version of Source where every field is
// filled in.
var (
	CompleteIdentitySpec = duckv1beta1.IdentitySpec{
		ServiceAccountName: "k8sServiceAccount",
	}

	CompletePubSubSpec = duckv1beta1.PubSubSpec{
		SourceSpec:   events.CompleteSourceSpec,
		IdentitySpec: CompleteIdentitySpec,
		Secret:       events.CompleteSecret,
		Project:      "project",
	}

	CompleteIdentityStatus = duckv1beta1.IdentityStatus{
		Status: duckv1.Status{
			ObservedGeneration: 7,
			Conditions: duckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		ServiceAccountName: "serviceAccountName",
	}

	CompletePubSubStatus = duckv1beta1.PubSubStatus{
		IdentityStatus: CompleteIdentityStatus,
		SinkURI:        &events.CompleteURL,
		CloudEventAttributes: []duckv1.CloudEventAttributes{
			{
				Type:   "type",
				Source: "source",
			},
		},
		ProjectID:      "projectID",
		TopicID:        "topicID",
		SubscriptionID: "subscriptionID",
	}
)
