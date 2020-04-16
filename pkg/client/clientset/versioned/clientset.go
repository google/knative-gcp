/*
Copyright 2020 Google LLC

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

// Code generated by client-gen. DO NOT EDIT.

package versioned

import (
	"fmt"

	eventsv1alpha1 "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/events/v1alpha1"
	eventsv1beta1 "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/events/v1beta1"
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/messaging/v1alpha1"
	policyv1alpha1 "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/policy/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/pubsub/v1alpha1"
	pubsubv1beta1 "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/pubsub/v1beta1"
	discovery "k8s.io/client-go/discovery"
	rest "k8s.io/client-go/rest"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	EventsV1alpha1() eventsv1alpha1.EventsV1alpha1Interface
	EventsV1beta1() eventsv1beta1.EventsV1beta1Interface
	MessagingV1alpha1() messagingv1alpha1.MessagingV1alpha1Interface
	PolicyV1alpha1() policyv1alpha1.PolicyV1alpha1Interface
	PubsubV1alpha1() pubsubv1alpha1.PubsubV1alpha1Interface
	PubsubV1beta1() pubsubv1beta1.PubsubV1beta1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	eventsV1alpha1    *eventsv1alpha1.EventsV1alpha1Client
	eventsV1beta1     *eventsv1beta1.EventsV1beta1Client
	messagingV1alpha1 *messagingv1alpha1.MessagingV1alpha1Client
	policyV1alpha1    *policyv1alpha1.PolicyV1alpha1Client
	pubsubV1alpha1    *pubsubv1alpha1.PubsubV1alpha1Client
	pubsubV1beta1     *pubsubv1beta1.PubsubV1beta1Client
}

// EventsV1alpha1 retrieves the EventsV1alpha1Client
func (c *Clientset) EventsV1alpha1() eventsv1alpha1.EventsV1alpha1Interface {
	return c.eventsV1alpha1
}

// EventsV1beta1 retrieves the EventsV1beta1Client
func (c *Clientset) EventsV1beta1() eventsv1beta1.EventsV1beta1Interface {
	return c.eventsV1beta1
}

// MessagingV1alpha1 retrieves the MessagingV1alpha1Client
func (c *Clientset) MessagingV1alpha1() messagingv1alpha1.MessagingV1alpha1Interface {
	return c.messagingV1alpha1
}

// PolicyV1alpha1 retrieves the PolicyV1alpha1Client
func (c *Clientset) PolicyV1alpha1() policyv1alpha1.PolicyV1alpha1Interface {
	return c.policyV1alpha1
}

// PubsubV1alpha1 retrieves the PubsubV1alpha1Client
func (c *Clientset) PubsubV1alpha1() pubsubv1alpha1.PubsubV1alpha1Interface {
	return c.pubsubV1alpha1
}

// PubsubV1beta1 retrieves the PubsubV1beta1Client
func (c *Clientset) PubsubV1beta1() pubsubv1beta1.PubsubV1beta1Interface {
	return c.pubsubV1beta1
}

// Discovery retrieves the DiscoveryClient
func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	if c == nil {
		return nil
	}
	return c.DiscoveryClient
}

// NewForConfig creates a new Clientset for the given config.
// If config's RateLimiter is not set and QPS and Burst are acceptable,
// NewForConfig will generate a rate-limiter in configShallowCopy.
func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		if configShallowCopy.Burst <= 0 {
			return nil, fmt.Errorf("Burst is required to be greater than 0 when RateLimiter is not set and QPS is set to greater than 0")
		}
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error
	cs.eventsV1alpha1, err = eventsv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.eventsV1beta1, err = eventsv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.messagingV1alpha1, err = messagingv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.policyV1alpha1, err = policyv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.pubsubV1alpha1, err = pubsubv1alpha1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.pubsubV1beta1, err = pubsubv1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	cs.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	return &cs, nil
}

// NewForConfigOrDie creates a new Clientset for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *Clientset {
	var cs Clientset
	cs.eventsV1alpha1 = eventsv1alpha1.NewForConfigOrDie(c)
	cs.eventsV1beta1 = eventsv1beta1.NewForConfigOrDie(c)
	cs.messagingV1alpha1 = messagingv1alpha1.NewForConfigOrDie(c)
	cs.policyV1alpha1 = policyv1alpha1.NewForConfigOrDie(c)
	cs.pubsubV1alpha1 = pubsubv1alpha1.NewForConfigOrDie(c)
	cs.pubsubV1beta1 = pubsubv1beta1.NewForConfigOrDie(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClientForConfigOrDie(c)
	return &cs
}

// New creates a new Clientset for the given RESTClient.
func New(c rest.Interface) *Clientset {
	var cs Clientset
	cs.eventsV1alpha1 = eventsv1alpha1.New(c)
	cs.eventsV1beta1 = eventsv1beta1.New(c)
	cs.messagingV1alpha1 = messagingv1alpha1.New(c)
	cs.policyV1alpha1 = policyv1alpha1.New(c)
	cs.pubsubV1alpha1 = pubsubv1alpha1.New(c)
	cs.pubsubV1beta1 = pubsubv1beta1.New(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &cs
}
