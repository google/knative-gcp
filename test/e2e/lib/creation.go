package lib

import (
    messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
    eventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

func (c *Client) CreateChannelOrFail(channel *messagingv1alpha1.Channel) {
    channels := c.KnativeGCP.MessagingV1alpha1().Channels(c.Namespace)
    _, err := channels.Create(channel)
    if err != nil {
        c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
    }
    c.Tracker.AddObj(channel)
}

func (c *Client) CreatePubSubOrFail(pubsub *eventsv1alpha1.PubSub) {
    pubsubs := c.KnativeGCP.EventsV1alpha1().PubSubs(c.Namespace)
    _, err := pubsubs.Create(pubsub)
    if err != nil {
        c.T.Fatalf("Failed to create pubsub %q: %v", pubsub.Name, err)
    }
    c.Tracker.AddObj(pubsub)
}
