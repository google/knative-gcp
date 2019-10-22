# Cloud Pub/Sub PullSubscription

This sample shows how to configure the Cloud Pub/Sub Pull Subscription. This
event source is most useful as a bridge from other Google Cloud services from a
Pub/Sub topic, such as
[Cloud Storage](https://cloud.google.com/storage/docs/pubsub-notifications),
[IoT Core](https://cloud.google.com/iot/docs/how-tos/devices) and
[Cloud Scheduler](https://cloud.google.com/scheduler/docs/creating#).

## Prerequisites

1. [Create a Pub/Sub enabled Service Account](../pubsub)

1. [Install Knative with GCP](../install).

## Deployment

1. Create a Cloud Pub/Sub Topic.

   ```shell
   export TOPIC_NAME=testing
   gcloud pubsub topics create $TOPIC_NAME
   ```

1. Update `TOPIC_NAME` in the [`pullsubscription.yaml`](pullsubscription.yaml)
   and apply it.

   If you're in the pullsubscription directory, you can replace `TOPIC_NAME` and
   apply in one command:

   ```shell
   sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
       kubectl apply --filename -
   ```

   If you are replacing `TOPIC_NAME` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename pullsubscription.yaml
   ```

1. [Optional] If not using GKE, or want to use a Pub/Sub topic from another
   project, uncomment and replace the
   [`MY_PROJECT` placeholder](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   in [`pullsubscription.yaml`](pullsubscription.yaml) and apply it.

   If you're in the pullsubscription directory, you can replace `MY_PROJECT` and
   `TOPIC_NAME` and then apply in one command:

   ```shell
   sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
   sed "s/\#project: MY_PROJECT/project: $PROJECT_ID/g" | \
       kubectl apply --filename -
   ```

   If you are replacing `MY_PROJECT` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename pullsubscription.yaml
   ```

1. Create a service that the Pub/Sub Subscription will sink into:

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
   type: com.google.cloud.pubsub.topic.publish
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
