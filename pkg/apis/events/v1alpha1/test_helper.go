package v1alpha1

import (
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/events"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// These variables are used to create a 'complete' version of Source where every field is
// filled in.
var (
	CompleteIdentitySpec = duckv1alpha1.IdentitySpec{
		ServiceAccountName: "k8sServiceAccount",
	}

	completePubSubSpec = duckv1alpha1.PubSubSpec{
		SourceSpec:   events.CompleteSourceSpec,
		IdentitySpec: CompleteIdentitySpec,
		Secret:       events.CompleteSecret,
		Project:      "project",
	}

	completeIdentityStatus = duckv1alpha1.IdentityStatus{
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

	completePubSubStatus = duckv1alpha1.PubSubStatus{
		IdentityStatus: completeIdentityStatus,
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
