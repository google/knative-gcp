# Cloud Storage Object Notification

## Overview

This sample shows how to Configure a `Storage` resource to deliver
Object Notifications for when a new object is added to
Google Cloud Storage (GCS). This is a simple example that uses
a single Google Service Account for manipulating both Pub/Sub resources
as well as GCS resources.

## Prerequisites

1. [Create a Pub/Sub enabled Service Account](../pubsub)

1. [Install Knative with GCP](../install).

1. Give that Service Account the `Storage Admin` role on your Google Cloud
   project:

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
   --member=serviceAccount:cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com \
   --role roles/storage.admin
   ```
1. Use an existing GCS Bucket, or create a new one. You can create a
   bucket either from the [Cloud Console](https://cloud.google/com/console) or by
   using [gsutil](https://cloud.google.com/storage/docs/gsutil/commands/mb)

   ```shell
   export BUCKET=<your bucket name>
   ```

1. Give Google Cloud Storage permissions to publish to GCP Pub/Sub.

   1. First find the Service Account that GCS uses to publish to Pub/Sub
      (Either using UI or using curl as shown below)

      1. Use the steps outlined in 
         [Cloud Console or the JSON API](https://cloud.google.com/storage/docs/getting-service-account)
         Assume the service account you found from above was
         `service-XYZ@gs-project-accounts.iam.gserviceaccount.com`, you'd do:
         `shell export GCS_SERVICE_ACCOUNT=service-XYZ@gs-project-accounts.iam.gserviceaccount.com`

      1. Use `curl` to fetch the email:

      ```shell
      export GCS_SERVICE_ACCOUNT=`curl -s -X GET -H "Authorization: Bearer \`GOOGLE_APPLICATION_CREDENTIALS=./cloudrunevents-pullsub.json gcloud auth application-default print-access-token\`" "https://www.googleapis.com/storage/v1/projects/$PROJECT_ID/serviceAccount" | grep email_address | cut -d '"' -f 4`
         ```

      1. Then grant rights to that Service Account to publish to GCP Pub/Sub.

         ```shell
         gcloud projects add-iam-policy-binding $PROJECT_ID \
           --member=serviceAccount:$GCS_SERVICE_ACCOUNT \
           --role roles/pubsub.publisher
         ```

## Deployment

1. Update `BUCKET` in the [`storage.yaml`](storage.yaml)
   and apply it.

   If you're in the storage directory, you can replace `BUCKET` and
   apply in one command:

   ```shell
   sed "s/\BUCKET/$BUCKET/g" storage.yaml | \
       kubectl apply --filename -
   ```

   If you are replacing `BUCKET` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename storage.yaml
   ```

1. [Optional] If not using GKE, or want to use a Pub/Sub topic from another
   project, uncomment and replace the
   [`MY_PROJECT` placeholder](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   in [`storage.yaml`](storage.yaml) and apply it.

   If you're in the storage directory, you can replace `MY_PROJECT` and
   `BUCKET` and then apply in one command:

   ```shell
   sed "s/\BUCKET/$BUCKET/g" pullsubscription.yaml | \
   sed "s/\#project: MY_PROJECT/project: $PROJECT_ID/g" | \
       kubectl apply --filename -
   ```

   If you are replacing `MY_PROJECT` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename storage.yaml
   ```

1. Create a service that the Storage notifications will sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Publish

Upload a file to your `Bucket`, either using the
[Cloud Console](https://cloud.google/com/console) or
gsutil:

```shell
gsutil cp storage.yaml gs://$BUCKET/testfilehere'
```

## Verify

Verify that the published message was sent by looking at the logs of the
service that this Storage notification sinks to.

1. To ensure the downstream pods to get started and receive our event,
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
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 0.3
  type: google.storage.object.finalize
  source: //storage.googleapis.com/buckets/vaikasvaikas-notification-test
  subject: testfilehere
  id: 710717795564381
  time: 2019-08-27T16:35:03.742Z
  schemaurl: https://raw.githubusercontent.com/google/knative-gcp/master/schemas/storage/schema.json
  datacontenttype: application/json
Extensions,
  error: unexpected end of JSON input
  notificationconfig: projects/_/buckets/vaikasvaikas-notification-test/notificationConfigs/174
  objectgeneration: 1566923702760643
  payloadformat: JSON_API_V1
Data,
  {
    "kind": "storage#object",
    "id": "vaikasvaikas-notification-test/testfilehere/1566923702760643",
    "selfLink": "https://www.googleapis.com/storage/v1/b/vaikasvaikas-notification-test/o/testfilehere",
    "name": "testfilehere",
    "bucket": "vaikasvaikas-notification-test",
    "generation": "1566923702760643",
    "metageneration": "1",
    "contentType": "application/octet-stream",
    "timeCreated": "2019-08-27T16:35:02.760Z",
    "updated": "2019-08-27T16:35:02.760Z",
    "storageClass": "STANDARD",
    "timeStorageClassUpdated": "2019-08-27T16:35:02.760Z",
    "size": "1432",
    "md5Hash": "RZkejFFNPn0p4GZyPKnqUg==",
    "mediaLink": "https://www.googleapis.com/download/storage/v1/b/vaikasvaikas-notification-test/o/testfilehere?generation=1566923702760643&alt=media",
    "crc32c": "JUiU0w==",
    "etag": "CMPpxdW9o+QCEAE="
  }
```

## What's Next

The Storage object implements what Knative Eventing considers to be an
"importer". This component can work alone, but it also works well when
[Knative Serving and Eventing](https://github.com/knative/docs) are installed in
the cluster.

## Cleaning Up

```shell
kubectl delete -f ./storage.yaml
kubectl delete -f ./event-display.yaml
gcloud projects remove-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com \
  --role roles/storage.admin
gcloud projects remove-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:$GCS_SERVICE_ACCOUNT \
  --role roles/pubsub.publisher
```

