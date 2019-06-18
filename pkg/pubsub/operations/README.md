# Pub/Sub Operations

Operations holds helper functions for creating and understanding Pub/Sub
related jobs that operate on Cloud Pub/Sub. 

### Using Directly

There are two Jobs that can be made:

- `NewSubscriptionOps` - Operations related to subscriptions.
- `NewTopicOps` - Operations related to topics.
    
Each Job accepts an Action, supported Actions are:

- `create`, will attempt to create the Pub/Sub Resource.
- `delete`, will attempt to delete the Pub/Sub Resource.


### Using in Reconcilers

These jobs are used with [PubSubBase](../pkg/reconciler/pubsub/reconciler.go) and provides the following methods:

- `EnsureSubscription` - Confirm or Create Subscription.
- `EnsureSubscriptionDeleted` - Delete Subscription.
- `EnsureTopic` - Confirm or Create Topic.
- `EnsureTopicDeleted` - Delete Topic.