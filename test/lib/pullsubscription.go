package lib

import (
	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"
	reconcilertestingv1beta1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PullSubscriptionConfig struct {
	SinkGVK              metav1.GroupVersionKind
	PullSubscriptionName string
	SinkName             string
	TopicName            string
	ServiceAccountName   string
}

func MakePullSubscriptionOrDie(client *Client, config PullSubscriptionConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1.PullSubscriptionOption, 0)
	so = append(so, reconcilertestingv1.WithPullSubscriptionSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1.WithPullSubscriptionTopic(config.TopicName))
	so = append(so, reconcilertestingv1.WithPullSubscriptionServiceAccount(config.ServiceAccountName))
	pullsubscription := reconcilertestingv1.NewPullSubscription(config.PullSubscriptionName, client.Namespace, so...)

	client.CreatePullSubscriptionOrFail(pullsubscription)

	client.Core.WaitForResourceReadyOrFail(config.PullSubscriptionName, PullSubscriptionV1TypeMeta)
}

func MakePullSubscriptionV1beta1OrDie(client *Client, config PullSubscriptionConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1beta1.PullSubscriptionOption, 0)
	so = append(so, reconcilertestingv1beta1.WithPullSubscriptionSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1beta1.WithPullSubscriptionTopic(config.TopicName))
	so = append(so, reconcilertestingv1beta1.WithPullSubscriptionServiceAccount(config.ServiceAccountName))
	pullsubscription := reconcilertestingv1beta1.NewPullSubscription(config.PullSubscriptionName, client.Namespace, so...)

	client.CreatePullSubscriptionV1beta1OrFail(pullsubscription)

	client.Core.WaitForResourceReadyOrFail(config.PullSubscriptionName, PullSubscriptionV1beta1TypeMeta)
}
