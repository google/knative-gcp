# Cloud Pub/Sub Channel

This sample shows how to configure a Channel backed by Cloud Pub/Sub. This is an
implementation of a [Knative Channel](TODO) intended to provide durable a messaging
solution.

## Prerequisites

1. [Create a Pub/Sub enabled Service Account](../pubsub)

1. [Install Knative with GCP](../install).

## Deployment

1. Create a `Channel`.

   **Note**: _Update `project` and `secret` if you are not using defaults._

   ```shell
   kubectl apply --filename channel.yaml
   ```

1. Validate that the Create a service that the Pub/Sub Subscription will sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Publish

Publish messages to your Cloud Pub/Sub Topic:

```shell
gcloud pubsub topics publish testing --message='{"Hello": "world"}'
```

## Verify

We will verify that the published message was sent by looking at the logs of the
service that this PullSubscription sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds.

   - You can check the status of the downstream pods with:

     ```shell
     kubectl get pods --selector app=event-display
     ```

     You should see at least one.

1. Inspect the logs of the service:

   ```shell
   kubectl logs --selector app=event-display -c user-container
   ```

You should see log lines similar to:

```shell
☁️ cloudevents.Event
Validation: valid
 Context Attributes,
   specversion: 0.2
   type: google.pubsub.topic.publish
   source: //pubsub.googleapis.com/PROJECT_ID/topics/TOPIC_NAME
   id: 9f9b0968-a15f-4e74-ac58-e8a1c4fa587d
   time: 2019-06-10T17:52:36.73Z
   contenttype: application/json
 Data,
   {
     "Hello": "world"
   }
```

For more information about the format of the `Data,` second of the message, see
the `data` field of
[PubsubMessage documentation](https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage).

For more information about CloudEvents, see the
[HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## What's Next

The Pub/Sub PullSubscription implements what Knative Eventing considers to be a
`source`. This component can work alone, but it also works well when
[Knative Serving and Eventing](https://github.com/knative/docs) are installed in
the cluster.

## Cleaning Up

1. Delete the Pub/Sub PullSubscription:

If you're in the pullsubscription directory, you can replace `TOPIC_NAME` and
delete in one command:

```shell
 sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
     kubectl delete --filename -
```

If you are replaced `TOPIC_NAME` manually, then make sure you delete the
resulting YAML:

```shell
kubectl apply --filename pullsubscription.yaml
```

1. Delete the service used as the sink:

```shell
kubectl delete --filename event-display.yaml
```
