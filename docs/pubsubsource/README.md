# Cloud Pub/Sub Source

This sample shows how to configure the PubSub event source. This event source is
most useful as a bridge from other GCP services, such as
[Cloud Storage](https://cloud.google.com/storage/docs/pubsub-notifications),
[IoT Core](https://cloud.google.com/iot/docs/how-tos/devices) and
[Cloud Scheduler](https://cloud.google.com/scheduler/docs/creating#).

## Prerequisites

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This sample will
   use a mix of `gcloud` and `kubectl` commands. The rest of the sample assumes
   that you've set the `$PROJECT_ID` environment variable to your Google Cloud
   project id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. [Install Cloud Run Events](../install).

1. Enable the `Cloud Pub/Sub API` on your project:

   ```shell
   gcloud services enable pubsub.googleapis.com
   ```

1. Create a
   [GCP Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
   This sample creates one service account for both registration and receiving
   messages, but you can also create a separate service account for receiving
   messages if you want additional privilege separation.

   1. Create a new service account named `cloudrunevents-pubsubsource` with the
      following command:
      ```shell
      gcloud iam service-accounts create cloudrunevents-pubsubsource
      ```
   1. Give that Service Account the `Pub/Sub Editor` role on your GCP project:
      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:cloudrunevents-pubsubsource@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/pubsub.editor
      ```
   1. Download a new JSON private key for that Service Account. **Be sure not to
      check this key into source control!**
      ```shell
      gcloud iam service-accounts keys create cloudrunevents-pubsubsource.json \
        --iam-account=cloudrunevents-pubsubsource@$PROJECT_ID.iam.gserviceaccount.com
      ```
   1. Create a secret on the kubernetes cluster with the downloaded key:

      ```shell
          # The secret should not already exist, so just try to create it.
          kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=cloudrunevents-pubsubsource.json
      ```
      
      `google-cloud-key` and `key.json` are pre-configured values in
      [`pubsub-source.yaml`](./pubsub-source.yaml).

## Deployment

1. Create a Cloud Pub/Sub Topic.

   ```shell
   export TOPIC_NAME=testing
   gcloud pubsub topics create $TOPIC_NAME
   ```

1. Update `TOPIC_NAME` in the [`pubsub-source.yaml`](./pubsub-source.yaml) and
   apply it.

   If you're in the samples directory, you can replace `TOPIC_NAME` and apply in
   one command:

   ```shell
   sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pubsub-source.yaml | \
       kubectl apply --filename -
   ```

   If you are replacing `TOPIC_NAME` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename pubsub-source.yaml
   ```

1. [Optional] If not using GKE, or want to use a Pub/Sub topic from another
   project, uncomment and replace the
   [`MY_GCP_PROJECT` placeholder](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   in [`pubsub-source.yaml`](./pubsub-source.yaml) and apply it.

   If you're in the samples directory, you can replace `MY_GCP_PROJECT` and
   `TOPIC_NAME` and then apply in one command:

   ```shell
   sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pubsub-source.yaml | \
   sed "s/\#project: MY_GCP_PROJECT/project: $PROJECT_ID/g" | \
       kubectl apply --filename -
   ```

   If you are replacing `MY_GCP_PROJECT` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename pubsub-source.yaml
   ```

1. Create a service that the Pub/Sub Source will sink into:

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
service the source sinks to.

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

The Pub/Sub Source implements what Knative Eventing considers to be a `source`.
This component can work alone, but it also works well when
[Knative Serving and Eventing](https://github.com/knative/docs) are installed in
the cluster.

## Cleaning Up

1. Delete the Pub/Sub Source:

If you're in the samples directory, you can replace `TOPIC_NAME` and delete in
one command:

```shell
 sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pubsub-source.yaml | \
     kubectl delete --filename -
```

If you are replaced `TOPIC_NAME` manually, then make sure you delete the
resulting YAML:

```shell
kubectl apply --filename pubsub-source.yaml
```

1. Delete the service used as the sink:

```shell
kubectl delete --filename event-display.yaml
```
