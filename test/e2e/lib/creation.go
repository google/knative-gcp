package lib

import messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"

func (c *Client) CreateChannelOrFail(channel *messagingv1alpha1.Channel) {
    channels := c.KnativeGCP.MessagingV1alpha1().Channels(c.Namespace)
    _, err := channels.Create(channel)
    if err != nil {
        c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
    }
    c.Tracker.AddObj(channel)
}

