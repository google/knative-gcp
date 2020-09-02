# CloudBuildSource Example

## Overview

This sample shows how to configure `CloudBuildSources`. The `CloudBuildSource`
fires a new event each time when your build's state changes, such as when your
build is created, when your build transitions to a working state, and when your
build completes.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Service Account for Data Plane](../../install/dataplane-service-account.md)

1. Enable the `Cloud Build API` and `Cloud Pub/Sub API`, on your project:

   ```shell
   gcloud services enable cloudbuild.googleapis.com
   gcloud services enable pubsub.googleapis.com
   ```

   The Pub/Sub topic to which Cloud Build publishes these build update messages
   is called `cloud-builds`. When you enable the `Cloud Build API` and
   `Cloud Pub/Sub API`, the `cloud-builds` topic will automatically be created
   for you.

## Deployment

1. Create a [`CloudBuildSource`](cloudbuildsource.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret which has the
      permission of `roles/pubsub.subscriber`.

   ```shell
   kubectl apply --filename cloudbuildsource.yaml
   ```

1. Create a [`Service`](event-display.yaml) that the Build notifications will
   sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Publish

You can refer to
[Cloud Build Quick Start: preparing source files](https://cloud.google.com/cloud-build/docs/quickstart-build#preparing_source_files)
to prepare source files. You can refer to
[Cloud Build Quick Start: build using dockerfile](https://cloud.google.com/cloud-build/docs/quickstart-build#build_using_dockerfile)
or
[Cloud Build Quick Start: build using a build config file](https://cloud.google.com/cloud-build/docs/quickstart-build#build_using_a_build_config_file)
to trigger a build.

## Verify

Once the build's state changes, we will verify that the published event was sent
by looking at the logs of the service that this CloudBuildSource sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait up to 60 seconds. You can check the status of the downstream pods with:

   ```shell
   kubectl get pods --selector app=event-display
   ```

   You should see at least one.

1. Inspect the logs of the service:

   ```shell
   kubectl logs --selector app=event-display -c user-container --tail=200
   ```

   You may see multiple events since the `CloudBuildSource` fires a new event
   each time when your build's state changes. You should see log lines similar
   to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: google.cloud.cloudbuild.build.v1.statusChanged
  source: //cloudbuild.googleapis.com/projects/PROJECT_ID/builds/BUILD_ID
  subject: SUCCESS
  id: 1085069104560583
  time: 2020-04-03T23:51:29.811Z
  dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/cloudbuild/v1/data.proto
  datacontenttype: application/json
Extensions,
  buildid: BUILD_ID
  knativecemode: binary
  status: SUCCESS
  traceparent: 01-ab169fd11cf9a5e308f4af4808259efe-675ce05f4k69ea3f-00
Data,
  {
    "id": "BUILD_ID",
    "projectId": "PROJECT_ID",
    "status": "SUCCESS",
    "source": {
      "storageSource": {
        "bucket": "PROJECT_ID_cloudbuild",
        "object": "source/1585957874.581832-90d126bbd24543ab9e01d5de6f8b9b7c.tgz",
        "generation": "1585957875127373"
      }
    },
    "steps": [
      {
        "name": "gcr.io/cloud-builders/docker",
        "args": [
          "build",
          "--network",
          "cloudbuild",
          "--no-cache",
          "-t",
          "gcr.io/PROJECT_ID/quickstart-image",
          "."
        ],
        "timing": {
          "startTime": "2020-04-03T23:51:21.250787950Z",
          "endTime": "2020-04-03T23:51:22.931602046Z"
        },
        "pullTiming": {
          "startTime": "2020-04-03T23:51:21.250787950Z",
          "endTime": "2020-04-03T23:51:21.258683997Z"
        },
        "status": "SUCCESS"
      }
    ],
    "results": {
      "images": [
        {
          "name": "gcr.io/PROJECT_ID/quickstart-image",
          "digest": "sha256:0c7566925d2cd9997a977887b43a9b751a83e7f14301584779f3986eb62c81c6",
          "pushTiming": {
            "startTime": "2020-04-03T23:51:22.999290127Z",
            "endTime": "2020-04-03T23:51:27.955285791Z"
          }
        },
        {
          "name": "gcr.io/PROJECT_ID/quickstart-image:latest",
          "digest": "sha256:0c7566925d2cd9997a977887b43a9b751a83e7f14301584779f3986eb62c81c6",
          "pushTiming": {
            "startTime": "2020-04-03T23:51:22.999290127Z",
            "endTime": "2020-04-03T23:51:27.955285791Z"
          }
        }
      ],
      "buildStepImages": [
        "sha256:eb8329874ddfcb260f282b471c8205e0b9a10f8d42c45efc8ab32221bce43402"
      ],
      "buildStepOutputs": [
        ""
      ]
    },
    "createTime": "2020-04-03T23:51:15.340709933Z",
    "startTime": "2020-04-03T23:51:16.287536045Z",
    "finishTime": "2020-04-03T23:51:29.281278Z",
    "timeout": "600s",
    "images": [
      "gcr.io/PROJECT_ID/quickstart-image"
    ],
    "artifacts": {
      "images": [
        "gcr.io/PROJECT_ID/quickstart-image"
      ]
    },
    "logsBucket": "gs://25338030513.cloudbuild-logs.googleusercontent.com",
    "sourceProvenance": {
      "resolvedStorageSource": {
        "bucket": "PROJECT_ID_cloudbuild",
        "object": "source/1585957874.581832-90d126bbd24543ab9e01d5de6f8b9b7c.tgz",
        "generation": "1585957875127373"
      },
      "fileHashes": {
        "gs://PROJECT_ID_cloudbuild/source/1585957874.581832-90d126bbd24543ab9e01d5de6f8b9b7c.tgz#1585957875127373": {
          "fileHash": [
            {
              "type": "MD5",
              "value": "ZUvek7aw2C45ZPcxLh2Vww=="
            }
          ]
        }
      }
    },
    "options": {
      "logging": "LEGACY"
    },
    "logUrl": "https://console.cloud.google.com/cloud-build/builds/BUILD_ID?project=25338030513",
    "timing": {
      "BUILD": {
        "startTime": "2020-04-03T23:51:20.489188704Z",
        "endTime": "2020-04-03T23:51:22.999129904Z"
      },
      "FETCHSOURCE": {
        "startTime": "2020-04-03T23:51:16.602170521Z",
        "endTime": "2020-04-03T23:51:20.489129604Z"
      },
      "PUSH": {
        "startTime": "2020-04-03T23:51:22.999289349Z",
        "endTime": "2020-04-03T23:51:27.955320728Z"
      }
    }
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
1. For integrating with Cloud Storage see the
   [Storage example](../../examples/cloudstoragesource/README.md).
1. For integrating with Cloud Scheduler see the
   [Scheduler example](../../examples/cloudschedulersource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `CloudBuildSource`

   ```shell
   kubectl delete -f ./cloudbuildsource.yaml
   ```

1. Delete the `Service`

   ```shell
   kubectl delete -f ./event-display.yaml
   ```
