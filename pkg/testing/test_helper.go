package testing

import (
	"net/url"
	"time"

	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knduckv1 "knative.dev/pkg/apis/duck/v1"
	knduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	knduckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
)

// These variables are used to create a 'complete' version of Source where every field is
// filled in.
var (
	trueVal           = true
	seconds           = int64(314)
	three             = int32(3)
	AckDeadline       = "ackDeadline"
	RetentionDuration = "30s"
	backoffPolicy     = eventingduckv1beta1.BackoffPolicyExponential
	backoffDelay      = "backoffDelay"

	CompleteObjectMeta = metav1.ObjectMeta{
		Name:            "name",
		GenerateName:    "generateName",
		Namespace:       "namespace",
		SelfLink:        "selfLink",
		UID:             "uid",
		ResourceVersion: "resourceVersion",
		Generation:      2012,
		CreationTimestamp: metav1.Time{
			Time: time.Unix(1, 1),
		},
		DeletionTimestamp: &metav1.Time{
			Time: time.Unix(2, 3),
		},
		DeletionGracePeriodSeconds: &seconds,
		Labels:                     map[string]string{"steel": "heart"},
		Annotations:                map[string]string{"New": "Cago"},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         "apiVersion",
				Kind:               "kind",
				Name:               "n",
				UID:                "uid",
				Controller:         &trueVal,
				BlockOwnerDeletion: &trueVal,
			},
		},
		Finalizers:  []string{"finalizer-1", "finalizer-2"},
		ClusterName: "clusterName",
	}

	CompleteURL = apis.URL{
		Scheme:     "scheme",
		Opaque:     "opaque",
		User:       url.User("user"),
		Host:       "host",
		Path:       "path",
		RawPath:    "rawPath",
		ForceQuery: false,
		RawQuery:   "rawQuery",
		Fragment:   "fragment",
	}

	CompleteDestination = knduckv1.Destination{
		Ref: &knduckv1.KReference{
			APIVersion: "apiVersion",
			Kind:       "kind",
			Namespace:  "namespace",
			Name:       "name",
		},
		URI: &CompleteURL,
	}

	CompleteSourceSpec = knduckv1.SourceSpec{
		Sink: CompleteDestination,
		CloudEventOverrides: &knduckv1.CloudEventOverrides{
			Extensions: map[string]string{"supers": "reckoners"},
		},
	}

	CompleteSecret = &v1.SecretKeySelector{
		LocalObjectReference: v1.LocalObjectReference{
			Name: "name",
		},
		Key:      "key",
		Optional: &trueVal,
	}

	CompleteV1alpha1IdentitySpec = duckv1alpha1.IdentitySpec{
		ServiceAccountName: "k8sServiceAccount",
	}

	CompleteV1alpha1PubSubSpec = duckv1alpha1.PubSubSpec{
		SourceSpec:   CompleteSourceSpec,
		IdentitySpec: CompleteV1alpha1IdentitySpec,
		Secret:       CompleteSecret,
		Project:      "project",
	}

	CompleteV1alpha1IdentityStatus = duckv1alpha1.IdentityStatus{
		Status: knduckv1.Status{
			ObservedGeneration: 7,
			Conditions: knduckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		ServiceAccountName: "serviceAccountName",
	}

	CompleteV1alpha1PubSubStatus = duckv1alpha1.PubSubStatus{
		IdentityStatus: CompleteV1alpha1IdentityStatus,
		SinkURI:        &CompleteURL,
		CloudEventAttributes: []knduckv1.CloudEventAttributes{
			{
				Type:   "type",
				Source: "source",
			},
		},
		ProjectID:      "projectID",
		TopicID:        "topicID",
		SubscriptionID: "subscriptionID",
	}

	CompleteV1alpha1PubSubStatusWithoutServiceAccountName = duckv1alpha1.PubSubStatus{
		IdentityStatus: CompleteV1alpha1IdentityStatusWithoutServiceAccountName,
		SinkURI:        &CompleteURL,
		CloudEventAttributes: []knduckv1.CloudEventAttributes{
			{
				Type:   "type",
				Source: "source",
			},
		},
		ProjectID:      "projectID",
		TopicID:        "topicID",
		SubscriptionID: "subscriptionID",
	}

	CompleteV1alpha1IdentityStatusWithoutServiceAccountName = duckv1alpha1.IdentityStatus{
		Status: knduckv1.Status{
			ObservedGeneration: 7,
			Conditions: knduckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	CompleteV1beta1IdentitySpec = duckv1beta1.IdentitySpec{
		ServiceAccountName: "k8sServiceAccount",
	}

	CompleteV1beta1PubSubSpec = duckv1beta1.PubSubSpec{
		SourceSpec:   CompleteSourceSpec,
		IdentitySpec: CompleteV1beta1IdentitySpec,
		Secret:       CompleteSecret,
		Project:      "project",
	}

	CompleteV1beta1IdentityStatus = duckv1beta1.IdentityStatus{
		Status: knduckv1.Status{
			ObservedGeneration: 7,
			Conditions: knduckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
		ServiceAccountName: "serviceAccountName",
	}

	CompleteV1beta1PubSubStatus = duckv1beta1.PubSubStatus{
		IdentityStatus: CompleteV1beta1IdentityStatus,
		SinkURI:        &CompleteURL,
		CloudEventAttributes: []knduckv1.CloudEventAttributes{
			{
				Type:   "type",
				Source: "source",
			},
		},
		ProjectID:      "projectID",
		TopicID:        "topicID",
		SubscriptionID: "subscriptionID",
	}

	CompleteV1alpha1AddressStatus = knduckv1alpha1.AddressStatus{
		Address: &knduckv1alpha1.Addressable{
			Addressable: knduckv1beta1.Addressable{
				URL: &CompleteURL,
			},
			Hostname: CompleteURL.Host,
		},
	}

	CompleteV1beta1AddressStatus = knduckv1beta1.AddressStatus{
		Address: &knduckv1beta1.Addressable{
			URL: &CompleteURL,
		},
	}

	CompleteV1AddressStatus = knduckv1.AddressStatus{
		Address: &knduckv1.Addressable{
			URL: &CompleteURL,
		},
	}

	CompleteV1alpha1Subscribable = &eventingduckv1alpha1.Subscribable{
		Subscribers: []eventingduckv1alpha1.SubscriberSpec{
			{
				UID:               "uid-1",
				Generation:        1,
				SubscriberURI:     &CompleteURL,
				ReplyURI:          &CompleteURL,
				DeadLetterSinkURI: &CompleteURL,
				Delivery: &eventingduckv1beta1.DeliverySpec{
					DeadLetterSink: &CompleteDestination,
					Retry:          &three,
					BackoffPolicy:  &backoffPolicy,
					BackoffDelay:   &backoffDelay,
				},
			},
		},
	}

	CompleteV1alpha1SubscribableTypeStatus = eventingduckv1alpha1.SubscribableTypeStatus{
		SubscribableStatus: &eventingduckv1alpha1.SubscribableStatus{
			Subscribers: []eventingduckv1beta1.SubscriberStatus{
				{
					UID:                "uid-1",
					ObservedGeneration: 1,
					Ready:              "ready-1",
					Message:            "message-1",
				},
				{
					UID:                "uid-2",
					ObservedGeneration: 2,
					Ready:              "ready-2",
					Message:            "message-2",
				},
			},
		},
	}

	CompleteV1IdentitySpec = duckv1.IdentitySpec{
		ServiceAccountName: "k8sServiceAccount",
	}

	CompleteV1PubSubSpec = duckv1.PubSubSpec{
		SourceSpec:   CompleteSourceSpec,
		IdentitySpec: CompleteV1IdentitySpec,
		Secret:       CompleteSecret,
		Project:      "project",
	}

	CompleteV1IdentityStatus = duckv1.IdentityStatus{
		Status: knduckv1.Status{
			ObservedGeneration: 7,
			Conditions: knduckv1.Conditions{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	CompleteV1PubSubStatus = duckv1.PubSubStatus{
		IdentityStatus: CompleteV1IdentityStatus,
		SinkURI:        &CompleteURL,
		CloudEventAttributes: []knduckv1.CloudEventAttributes{
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
