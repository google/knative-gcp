# Cloud Pub/Sub Channel

This sample shows how to configure a Channel backed by Cloud Pub/Sub. This is an
implementation of a
[Knative Channel](https://github.com/knative/eventing/blob/master/docs/spec/channel.md)
intended to provide a durable messaging solution.

## Prerequisites

1. [Create a Pub/Sub enabled Service Account](../pubsub)

1. [Install Knative with GCP](../install).

1. Install Knative Eventing,

   1. To install Knative Eventing, first install the CRDs by running the kubectl
      apply command once with the -l knative.dev/crd-install=true flag. This
      prevents race conditions during the install, which cause intermittent
      errors:

   ```shell
      kubectl apply --selector knative.dev/crd-install=true \
      --filename https://github.com/knative/eventing/releases/download/v0.8.0/release.yaml
   ```

   ```shell
      kubectl apply  --filename https://github.com/knative/eventing/releases/download/v0.8.0/release.yaml
   ```

## Deployment

1. Create a `Channel`.

   **Note**: _Update `project` and `secret` if you are not using defaults._

   ```shell
   kubectl apply --filename channel.yaml
   ```

   After a moment, the demo channel should become ready.

   ```shell
   kubectl get pschan demo
   ```

1. Create a subscriber, `event-display`.

   ```shell
   kubectl apply --filename event-display.yaml
   ```

1. Create a `Subscription`.

   ```shell
   kubectl apply --filename subscription.yaml
   ```

   After a moment, the subscription will become ready.

   ```shell
   kubectl get Subscription demo
   ```

1. Create an event source.

   ```shell
   kubectl apply --filename source.yaml
   ```

   This will send an event through the `demo` channel every minute on the
   minute.

## Verify

This results in the following:

```
[hello-world] --> [demo channel] -> [event-display]
```

1. Inspect the logs of the `event-display` pod:

   ```shell
   kubectl logs --selector app=event-display -c user-container
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 0.3
  type: dev.knative.cronjob.event
  source: /apis/v1/namespaces/default/cronjobsources/hello-world
  id: 37a8a186-acc0-4c63-b1ad-a8dac9caf288
  time: 2019-08-26T20:48:00.000475893Z
  datacontenttype: application/json
Data,
  {
    "hello": "world"
  }
```

These events are generated from the `hello-world` CronJobSource, sent through
the `demo` Channel and delivered to the `event-display` via the `demo`
Subscription.

## What's Next

The `Channel` implements what Knative Eventing considers to be a `channelable`.
This component can work alone, but it also works well when
[Knative Serving and Eventing](https://github.com/knative/docs) are installed in
the cluster.

## Cleaning Up

1. Delete the resources:

```shell
kubectl delete \
  --filename channel.yaml \
  --filename event-display.yaml \
  --filename subscription.yaml \
  --filename source.yaml
```
