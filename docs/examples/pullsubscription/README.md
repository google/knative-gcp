# PullSubscription Example

This sample shows how to configure `PullSubscriptions`, which is our Kubernetes
object to represent Cloud Pub/Sub subscriptions. This resource can be considered
an implementation detail of the
[CloudPubSubSource](../../examples/cloudpubsubsource/README.md), and users
should rather use the latter if they want to bridge events from Pub/Sub into
their clusters. As opposed to the  
`CloudPubSubSource`, which sends events using the Push-compatible format, this
does so using a Pull format.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

## Deployment

1. Create a Cloud Pub/Sub Topic.

   ```shell
   export TOPIC_NAME=testing
   gcloud pubsub topics create $TOPIC_NAME
   ```

1. Update `googleServiceAccount` / `secret` in the
   [`pullsubscription.yaml`](pullsubscription.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

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

1. **[Optional]** If you are not using GKE, or want to use a Pub/Sub topic from
   another project, uncomment and replace the `MY_PROJECT` placeholder in
   [`pullsubscription.yaml`](pullsubscription.yaml) and apply it. Note that the
   Service Account during the
   [installation](../../install/install-knative-gcp.md) step should be able to
   manage [multiple projects](../../install/managing-multiple-projects.md).

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

1. Create a [`Service`](event-display.yaml) that the `PullSubscription` will
   sink into:

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
service that this `PullSubscription` sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds.

   - You can check the status of the downstream pods with:

     ```shell
     kubectl get pods --selector app=event-display
     ```

     You should see at least one.

1. Inspect the logs of the service:

   ```shell
   kubectl logs --selector app=event-display -c user-container --tail=200
   ```

You should see log lines similar to:

```shell
☁️ cloudevents.Event
Validation: valid
 Context Attributes,
   specversion: 1.0
   type: google.cloud.pubsub.topic.v1.messagePublished
   source: //pubsub.googleapis.com/PROJECT_ID/topics/TOPIC_NAME
   id: 9f9b0968-a15f-4e74-ac58-e8a1c4fa587d
   time: 2019-06-10T17:52:36.73Z
   contenttype: application/octet-stream
 Data,
   {
     "Hello": "world"
   }
```

For more information about the format of the `Data` see the `data` field of
[PubsubMessage documentation](https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage).

## Troubleshooting

You may have issues receiving desired CloudEvent. Please use
[Authentication Mechanism Troubleshooting](../../how-to/authentication-mechanism-troubleshooting.md)
to check if it is due to an auth problem.

## What's next

1. For more details on Cloud Pub/Sub formats refer to the
   [Subscriber overview guide](https://cloud.google.com/pubsub/docs/subscriber).
1. For a higher-level construct to interact with Cloud Pub/Sub that sends
   Push-compatible format events, see the
   [PubSub example](../../examples/cloudpubsubsource/README.md).
1. For integrating with Cloud Storage see the
   [Storage example](../../examples/cloudstoragesource/README.md).
1. For integrating with Cloud Scheduler see the
   [Scheduler example](../../examples/cloudschedulersource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `PullSubscription`

   If you're in the pullsubscription directory, you can replace `TOPIC_NAME` and
   delete in one command:

   ```shell
    sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
        kubectl delete --filename -
   ```

   If you replaced `TOPIC_NAME` manually, then make sure you delete the
   resulting YAML:

   ```shell
   kubectl delete --filename pullsubscription.yaml
   ```

1. Delete the `Service` used as the sink:

   ```shell
   kubectl delete --filename event-display.yaml
   ```
