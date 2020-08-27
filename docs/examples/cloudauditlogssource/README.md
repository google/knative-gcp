# CloudAuditLogsSource Example

## Overview

This sample shows how to Configure a `CloudAuditLogsSource` resource to read
data from [Cloud Audit Logs](https://cloud.google.com/logging/docs/audit/) and
directly publish to the underlying transport (Pub/Sub), in CloudEvents format.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

1. Enable the `Cloud Audit Logs API` on your project:

   ```shell
   gcloud services enable logging.googleapis.com
   gcloud services enable stackdriver.googleapis.com
   ```

## Deployment

1. Create a [`CloudAuditLogsSource`](cloudauditlogssource.yaml). This
   `CloudAuditLogsSource` will emit Cloud Audit Log Entries for the creation of
   Pub/Sub topics.  
   You can change the `serviceName`, `methodName` and `resourceName` (see
   detailed description
   [here](https://cloud.google.com/logging/docs/reference/audit/auditlog/rest/Shared.Types/AuditLog))
   to select the Cloud Audit Log Entries you want to view.

   | CloudAuditLogsSource |  Audit Log Entry Fields   |
   | :------------------: | :-----------------------: |
   |   spec.serviceName   | protoPayload.serviceName  |
   |   spec.methodName    |  protoPayload.methodName  |
   |  spec.resourceName   | protoPayload.resourceName |

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

   ```shell
   kubectl apply --filename cloudauditlogssource.yaml
   ```

1. Create a [`Service`](event-display.yaml) that the CloudAuditLogsSource will
   sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Publish

Create a GCP PubSub Topic:

```shell
gcloud pubsub topics create test-auditlogs-source
```

## Verify

We will verify that the published event was sent by looking at the logs of the
service that this CloudAuditLogsSource sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds. You can check the status of the downstream pods with:

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
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: google.cloud.audit.log.v1.written
  source: //cloudaudit.googleapis.com/projects/test-project/logs/activity
  subject: pubsub.googleapis.com/projects/test-project/topics/test-auditlogs-source
  id: b2aaf4b545f42b4156d6f5fe7cca24b4
  time: 2020-06-30T16:14:47.593398572Z
  dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/audit/v1/data.proto
  datacontenttype: application/json
Extensions,
  knativearrivaltime: 2020-06-30T16:14:50.476091448Z
  knsourcetrigger: link0.26303396786461575
  methodname: google.pubsub.v1.Publisher.CreateTopic
  resourcename: projects/test-project/topics/test-auditlogs-source
  servicename: pubsub.googleapis.com
  traceparent: 00-e4e9a3d06604b0b080bf20f622f52d36-8b7a0438e6e607ba-00
Data,
  {
    "insertId": "9frck8cf9j",
    "logName": "projects/test-project/logs/cloudaudit.googleapis.com%2Factivity",
    "protoPayload": {
      "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
      "authenticationInfo": {
        "principalEmail": "robot@test-project.iam.gserviceaccount.com",
        "principalSubject": "user:robot@test-project.iam.gserviceaccount.com",
        "serviceAccountKeyName": "//iam.googleapis.com/projects/test-project/serviceAccounts/robot@test-project.iam.gserviceaccount.com/keys/90f662482321f1ca8e82ea675b1a1c30c1fe681f"
      },
      "authorizationInfo": [
        {
          "granted": true,
          "permission": "pubsub.topics.create",
          "resource": "projects/test-project",
          "resourceAttributes": {}
        }
      ],
      "methodName": "google.pubsub.v1.Publisher.CreateTopic",
      "request": {
        "@type": "type.googleapis.com/google.pubsub.v1.Topic",
        "name": "projects/test-project/topics/test-auditlogs-source"
      },
      "requestMetadata": {
        "callerIp": "192.168.0.1",
        "callerNetwork": "//compute.googleapis.com/projects/google.com:my-laptop/global/networks/__unknown__",
        "callerSuppliedUserAgent": "google-cloud-sdk",
        "destinationAttributes": {},
        "requestAttributes": {
          "auth": {},
          "time": "2020-06-30T16:14:47.600710407Z"
        }
      },
      "resourceLocation": {
        "currentLocations": [
          "asia-east1",
          "asia-northeast1",
          "asia-southeast1",
          "australia-southeast1",
          "europe-north1",
          "europe-west1",
          "europe-west2",
          "europe-west3",
          "europe-west4",
          "us-central1",
          "us-central2",
          "us-east1",
          "us-east4",
          "us-west1",
          "us-west2",
          "us-west3",
          "us-west4"
        ]
      },
      "resourceName": "projects/test-project/topics/test-auditlogs-source",
      "response": {
        "@type": "type.googleapis.com/google.pubsub.v1.Topic",
        "messageStoragePolicy": {
          "allowedPersistenceRegions": [
            "asia-east1",
            "asia-northeast1",
            "asia-southeast1",
            "australia-southeast1",
            "europe-north1",
            "europe-west1",
            "europe-west2",
            "europe-west3",
            "europe-west4",
            "us-central1",
            "us-central2",
            "us-east1",
            "us-east4",
            "us-west1",
            "us-west2",
            "us-west3",
            "us-west4"
          ]
        },
        "name": "projects/test-project/topics/test-auditlogs-source"
      },
      "serviceName": "pubsub.googleapis.com"
    },
    "receiveTimestamp": "2020-06-30T16:14:48.401489148Z",
    "resource": {
      "labels": {
        "project_id": "test-project",
        "topic_id": "projects/test-project/topics/test-auditlogs-source"
      },
      "type": "pubsub_topic"
    },
    "severity": "NOTICE",
    "timestamp": "2020-06-30T16:14:47.593398572Z"
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
1. For integrating with Cloud Build see the
   [Build example](../../examples/cloudbuildsource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `CloudAuditLogsSource`

   ```shell
   kubectl delete -f ./cloudauditlogssource.yaml
   ```

1. Delete the `Service`

   ```shell
   kubectl delete -f ./event-display.yaml
   ```
