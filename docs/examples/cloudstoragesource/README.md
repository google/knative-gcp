# CloudStorageSource Example

## Overview

This sample shows how to Configure a `CloudStorageSource` resource to deliver
Object Notifications for when a new object is added to Google Cloud Storage
(GCS).

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

1. Enable the `Cloud Storage API` on your project and give Google Cloud Storage
   permissions to publish to GCP Pub/Sub. Currently, we support two methods:
   automated script or manual

   - Option 1 (Recommended, via automated scripts): Apply
     [init_cloud_storage_source.sh](../../../hack/init_cloud_storage_source.sh):

     ```shell
     ./hack/init_cloud_storage_source.sh
     ```

     **_Note_**: This script will need one parameter

     1. `PROJECT_ID`: an optional parameter to specify the project to use,
        default to `gcloud config get-value project` If you want to specify the
        parameter `PROJECT_ID` instead of using the default one,

     ```shell
     ./hack/init_cloud_storage_source.sh [PROJECT_ID]
     ```

   - Option 2 (manual):

   1. Enable the `Cloud Storage API` on your project:
      ```shell
      gcloud services enable storage-component.googleapis.com
      gcloud services enable storage-api.googleapis.com
      ```
   1. Give Google Cloud Storage permissions to publish to GCP Pub/Sub.

      1. First find the Service Account that GCS uses to publish to Pub/Sub
         (Either using UI or using curl as shown below)

      - Option 1: Use the steps outlined in
        [Cloud Console or the JSON API](https://cloud.google.com/storage/docs/getting-service-account)
        Assume the service account you found from above was
        `service-XYZ@gs-project-accounts.iam.gserviceaccount.com`, you'd do:
        ```shell
        export GCS_SERVICE_ACCOUNT=service-XYZ@gs-project-accounts.iam.gserviceaccount.com
        ```
      - Option 2: Use `curl` to fetch the email:
        ```shell
        export GCS_SERVICE_ACCOUNT=`curl -s -X GET -H "Authorization: Bearer \`GOOGLE_APPLICATION_CREDENTIALS=./cre-dataplane.json \
          gcloud auth application-default print-access-token\`" \
          "https://www.googleapis.com/storage/v1/projects/$PROJECT_ID/serviceAccount" \
          | grep email_address | cut -d '"' -f 4`
        ```

   1. Then grant rights to that Service Account to publish to GCP Pub/Sub.
      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:$GCS_SERVICE_ACCOUNT \
        --role roles/pubsub.publisher
      ```

1. Use an existing GCS Bucket, or create a new one. You can create a bucket
   either from the [Cloud Console](https://cloud.google.com/console) or by using
   [gsutil](https://cloud.google.com/storage/docs/gsutil/commands/mb)

   ```shell
   export BUCKET=<your bucket name>
   ```

## Deployment

1. Update `serviceAccountName` / `secret` in the
   [`cloudstoragesource.yaml`](cloudstoragesource.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

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
   kubectl logs --selector app=event-display -c user-container --tail=200
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: google.cloud.storage.object.v1.finalized
  source: //storage.googleapis.com/projects/_/buckets/knativegcp-rc1
  subject: objects/myimage.jpeg
  id: 1313899854146765
  time: 2020-06-30T16:26:12.334Z
  dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
  datacontenttype: application/json
Extensions,
  knativearrivaltime: 2020-06-30T16:26:12.860787657Z
  knsourcetrigger: link0.17754831512278457
  traceparent: 00-dfa40189f717da88565d7f489e1cb356-fbe2f82875154c65-00
Data,
  {
    "kind": "storage#object",
    "id": "knativegcp-rc1/myimage.jpeg/1593534371944198",
    "selfLink": "https://www.googleapis.com/storage/v1/b/knativegcp-rc1/o/myimage.jpeg",
    "name": "myimage.jpeg",
    "bucket": "knativegcp-rc1",
    "generation": "1593534371944198",
    "metageneration": "1",
    "contentType": "image/jpeg",
    "timeCreated": "2020-06-30T16:26:11.943Z",
    "updated": "2020-06-30T16:26:11.943Z",
    "storageClass": "STANDARD",
    "timeStorageClassUpdated": "2020-06-30T16:26:11.943Z",
    "size": "12268",
    "md5Hash": "Gpx8dT1pF2lbIvLll9Ynyw==",
    "mediaLink": "https://www.googleapis.com/download/storage/v1/b/knativegcp-rc1/o/myimage.jpeg?generation=1593534371944198&alt=media",
    "crc32c": "zO8qSQ==",
    "etag": "CIautZH6qeoCEAE="
  }
```

## Troubleshooting

You may have issues receiving desired CloudEvent. Please use
[Authentication Mechanism Troubleshooting](../../how-to/authentication-mechanism-troubleshooting.md)
to check if it is due to an auth problem.

## What's Next

1. For more details on Cloud Pub/Sub formats refer to the
   [Subscriber overview guide](https://cloud.google.com/pubsub/docs/subscriber).
1. For integrating with Cloud Pub/Sub, see the
   [PubSub example](../../examples/cloudpubsubsource/README.md).
1. For integrating with Cloud Scheduler see the
   [Scheduler example](../../examples/cloudschedulersource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For integrating with Cloud Build see the
   [Build example](../../examples/cloudbuildsource/README.md).
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
