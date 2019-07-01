# Cloud Pub/Sub PullSubscription

This sample shows how to configure the Cloud Pub/Sub Pull Subscription. This
event source is most useful as a bridge from other Google Cloud services from a
Pub/Sub topic, such as
[Cloud Storage](https://cloud.google.com/storage/docs/pubsub-notifications),
[IoT Core](https://cloud.google.com/iot/docs/how-tos/devices) and
[Cloud Scheduler](https://cloud.google.com/scheduler/docs/creating#).

## Prerequisites

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This sample will
   use a mix of `gcloud` and `kubectl` commands.

1. Save the Google Cloud Project's ID to an environment variable and ensure `gcloud` is using that
   project.
   ```shell
   export PROJECT_ID="my-gcp-project" # Replace 'my-gcp-project' with your project's ID.
   gcloud config set project $PROJECT_ID
   ```

1. [Install Cloud Run Events](../install).

1. Enable the `Cloud Pub/Sub API` on your project:

   ```shell
   gcloud services enable pubsub.googleapis.com
   ```

1. [TODO - This is only required outside of GKE] Create a
   [Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
   This sample creates one service account for both registration and receiving
   messages, but you can also create a separate service account for receiving
   messages if you want additional privilege separation.

   1. Create a new service account named `cloudrunevents-pullsubscription` with
      the following command:
      ```shell
      gcloud iam service-accounts create cloudrunevents-pullsubscription
      ```
   1. Give that Service Account the `Pub/Sub Editor` role on your Google Cloud
      project:
      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:cloudrunevents-pullsubscription@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/pubsub.editor
      ```
   1. Download a new JSON private key for that Service Account. **Be sure not to
      check this key into source control!**
      ```shell
      gcloud iam service-accounts keys create cloudrunevents-pullsubscription.json \
        --iam-account=cloudrunevents-pullsubscription@$PROJECT_ID.iam.gserviceaccount.com
      ```
   1. Create a secret on the kubernetes cluster with the downloaded key:

      ```shell
          # The secret should not already exist, so just try to create it.
          kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=cloudrunevents-pullsubscription.json
      ```

      `google-cloud-key` and `key.json` are pre-configured values in
      [`pullsubscription.yaml`](pullsubscription.yaml).

## Deployment

1. Create a Cloud Pub/Sub Topic.

   ```shell
   export TOPIC_NAME=testing
   gcloud pubsub topics create $TOPIC_NAME
   ```

1. Replace the placeholders with real values.

      1. Placeholders:
          - `TOPIC_NAME` - Must be replaced.
          - `MY_PROJECT` - Replaced only if you are running outside of Google Kubernetes Engine.
            Note that if you are replacing this, then you should also uncomment the `#secret:` line.
      1.  Running inside Google Kubernetes Engine:

           If you're in the samples directory, you can replace `TOPIC_NAME` and apply in
           one command:

           ```shell
           sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
               kubectl apply --filename -
           ```

           If you are replacing `TOPIC_NAME` manually, then make sure you apply the
           resulting YAML:

           ```shell
           kubectl apply --filename pullsubscription.yaml
           ```
      1. Running outside Google Kubenetes Engine or want to use a Pub/Sub Topic from a different
         project than your GKE cluster:

           If you're in the samples directory, you can replace `MY_PROJECT`,
           `TOPIC_NAME`, uncomment `#secret:`, and then apply in a single command:

           ```shell
           sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
           sed "s/\#project: MY_PROJECT/project: $PROJECT_ID/g" | \
           sed "s/\#secret: /secret: /g" | \
               kubectl apply --filename -
           ```

           If you are replacing `TOPIC_NAME`, `MY_PROJECT`, and uncommenting the secret manually,
           then make sure you apply the resulting YAML:

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

For more information about the format of the `Data`, section of the message, see
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

If you're in the samples directory, you can replace `TOPIC_NAME` and delete in
one command:

```shell
 sed "s/\TOPIC_NAME/$TOPIC_NAME/g" pullsubscription.yaml | \
     kubectl delete --filename -
```

If you are replaced `TOPIC_NAME` manually, then make sure you delete the
resulting YAML:

```shell
kubectl delete --filename pullsubscription.yaml
```

1. Delete the service used as the sink:

```shell
kubectl delete --filename event-display.yaml
```
