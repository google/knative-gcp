# CloudStorageSource Example

## Overview

This sample shows how to Configure a `CloudStorageSource` resource to deliver
Object Notifications for when a new object is added to Google Cloud Storage
(GCS).

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md)

1. Enable the `Cloud Storage API` on your project:

   ```shell
   gcloud services enable storage-component.googleapis.com
   gcloud services enable storage-api.googleapis.com
   ```

1. Use an existing GCS Bucket, or create a new one. You can create a bucket
   either from the [Cloud Console](https://cloud.google.com/console) or by using
   [gsutil](https://cloud.google.com/storage/docs/gsutil/commands/mb)

   ```shell
   export BUCKET=<your bucket name>
   ```

1. Give Google Cloud Storage permissions to publish to GCP Pub/Sub.

   1. First find the Service Account that GCS uses to publish to Pub/Sub (Either
      using UI or using curl as shown below)

      1. Use the steps outlined in
         [Cloud Console or the JSON API](https://cloud.google.com/storage/docs/getting-service-account)
         Assume the service account you found from above was
         `service-XYZ@gs-project-accounts.iam.gserviceaccount.com`, you'd do:
         `shell export GCS_SERVICE_ACCOUNT=service-XYZ@gs-project-accounts.iam.gserviceaccount.com`

      1. Use `curl` to fetch the email:

      ```shell
      export GCS_SERVICE_ACCOUNT=`curl -s -X GET -H "Authorization: Bearer \`GOOGLE_APPLICATION_CREDENTIALS=./cre-pubsub.json gcloud auth application-default print-access-token\`" "https://www.googleapis.com/storage/v1/projects/$PROJECT_ID/serviceAccount" | grep email_address | cut -d '"' -f 4`
      ```

      1. Then grant rights to that Service Account to publish to GCP Pub/Sub.

         ```shell
         gcloud projects add-iam-policy-binding $PROJECT_ID \
           --member=serviceAccount:$GCS_SERVICE_ACCOUNT \
           --role roles/pubsub.publisher
         ```

## Deployment

1. Update `googleServiceAccount` / `secret` in the
   [`cloudstoragesource.yaml`](cloudstoragesource.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `googleServiceAccount` with the Pub/Sub enabled service account you
      created in
      [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md).

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

1. Update `BUCKET` in the [`cloudstoragesource.yaml`](cloudstoragesource.yaml)
   and apply it.

   If you're in the storage directory, you can replace `BUCKET` and apply in one
   command:

   ```shell
   sed "s/BUCKET/$BUCKET/g" cloudstoragesource.yaml | \
       kubectl apply --filename -
   ```

   If you are replacing `BUCKET` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename cloudstoragesource.yaml
   ```

1. [Optional] If not using GKE, or want to use a Pub/Sub topic from another
   project, uncomment and replace the `MY_PROJECT` placeholder in
   [`cloudstoragesource.yaml`](cloudstoragesource.yaml) and apply it. Note that
   the Service Account during the
   [installation](../../install/install-knative-gcp.md) step should be able to
   manage [multiple projects](../../install/managing-multiple-projects.md).

   If you're in the storage directory, you can replace `MY_PROJECT` and `BUCKET`
   and then apply in one command:

   ```shell
   sed "s/BUCKET/$BUCKET/g" pullsubscription.yaml | \
   sed "s/\#project: MY_PROJECT/project: $PROJECT_ID/g" | \
       kubectl apply --filename -
   ```

   If you are replacing `MY_PROJECT` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename cloudstoragesource.yaml
   ```

1. Create a [`Service`](event-display.yaml) that the Storage notifications will
   sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Publish

Upload a file to your BUCKET, either using the
[Cloud Console](https://cloud.google.com/console) or gsutil:

```shell
gsutil cp cloudstoragesource.yaml gs://$BUCKET/testfilehere
```

## Verify

Verify that the published message was sent by looking at the logs of the service
that this Storage notification sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds. You can check the status of the downstream pods with:

   ```shell
   kubectl get pods --selector app=event-display
   ```

   You should see at least one.

1. Inspect the logs of the `Service`:

   ```shell
   kubectl logs --selector app=event-display -c user-container
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: com.google.cloud.storage.object.finalize
  source: //storage.googleapis.com/buckets/vaikasvaikas-notification-test
  subject: testfilehere
  id: 710717795564381
  time: 2019-08-27T16:35:03.742Z
  schemaurl: https://raw.githubusercontent.com/google/knative-gcp/master/schemas/storage/schema.json
  datacontenttype: application/json
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

1. For integrating with Cloud Pub/Sub, see the
   [PubSub example](../../examples/cloudpubsubsource/README.md).
1. For integrating with Cloud Scheduler see the
   [Scheduler example](../../examples/cloudschedulersource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `CloudStorageSource`

   ```shell
   kubectl delete -f ./cloudstoragesource.yaml
   ```

1. Delete the `Service`

   ```shell
   kubectl delete -f ./event-display.yaml
   ```
