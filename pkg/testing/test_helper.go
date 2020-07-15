package testing

import (
	"net/url"
	"time"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// These variables are used to create a 'complete' version of Source where every field is
// filled in.
var (
	trueVal = true
	seconds = int64(314)

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

	CompleteDestination = duckv1.Destination{
		Ref: &duckv1.KReference{
			APIVersion: "apiVersion",
			Kind:       "kind",
			Namespace:  "namespace",
			Name:       "name",
		},
		URI: &CompleteURL,
	}

	CompleteSourceSpec = duckv1.SourceSpec{
		Sink: CompleteDestination,
		CloudEventOverrides: &duckv1.CloudEventOverrides{
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

	CompleteV1alpha1PubSubStatus = duckv1alpha1.PubSubStatus{
		IdentityStatus: CompleteV1alpha1IdentityStatus,
		SinkURI:        &CompleteURL,
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

	CompleteV1beta1PubSubStatus = duckv1beta1.PubSubStatus{
		IdentityStatus: CompleteV1beta1IdentityStatus,
		SinkURI:        &CompleteURL,
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
