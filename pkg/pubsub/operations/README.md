# Pub/Sub Operations

Operations holds helper functions for creating and understanding Pub/Sub related
jobs that operate on Cloud Pub/Sub.

## Usage

### Directly

There are two Jobs that can be made:

- `NewSubscriptionOps` - Operations related to subscriptions.
- `NewTopicOps` - Operations related to topics.

Each Job accepts an Action, supported Actions are:

- `create`, will attempt to create the Pub/Sub Resource.
- `delete`, will attempt to delete the Pub/Sub Resource.

### Within Reconcilers

These jobs are used with PubSubBase and provides the following methods:

- `EnsureSubscription` - Confirm or Create Subscription.
- `EnsureSubscriptionDeleted` - Delete Subscription.
- `EnsureTopic` - Confirm or Create Topic.
- `EnsureTopicDeleted` - Delete Topic.

## Why Jobs?

The `Jobs` are designed to be run in the namespace in which the
`PullSubscription` exists with the auth provided. The `Job` has to run local to
the resource to avoid copying service accounts or secrets into the
`cloud-run-events` namespace.

The controller will re-reconcile the resource if the create job is deleted, this
could be used as a healing operation by the operator if the Cloud Pub/Sub
subscription is deleted using `gcloud` or the Cloud Console by mistake.
