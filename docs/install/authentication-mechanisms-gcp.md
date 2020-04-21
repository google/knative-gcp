
# Manually Configure Authentication Mechanism for GCP

For both methods (Workload Identity and Kubernetes Secret), the first
manual step is creating a
[Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
with the appropriate permissions needed for the control plane to manage
native GCP resources.

You need to create a new Google Cloud Service Account named
`cloud-run-events` with the following command:

```shell
gcloud iam service-accounts create cloud-run-events
```

Then, give that Google Cloud Service Account permissions on your project.
The actual permissions needed will depend on the resources you are
planning to use. The Table below enumerates such permissions:

|    Resource / Functionality     |                                     Roles                                      |
| :-----------------------------: | :----------------------------------------------------------------------------: |
|        CloudPubSubSource        |                              roles/pubsub.editor                               |
|       CloudStorageSource        |                              roles/storage.admin                               |
|      CloudSchedulerSource       |                           roles/cloudscheduler.admin                           |
|      CloudAuditLogsSource       | roles/pubsub.admin, roles/logging.configWriter, roles/logging.privateLogViewer |
|        CloudBuildSource         |                            roles/pubsub.subscriber                             |
|             Channel             |                              roles/pubsub.editor                               |
|        PullSubscription         |                              roles/pubsub.editor                               |
|              Topic              |                              roles/pubsub.editor                               |
| Workload Identity in Data Plane |                         roles/iam.serviceAccountAdmin                          |

In this guide, and for the sake of simplicity, we will just grant
`roles/owner` privileges to the Google Cloud Service Account, which
encompasses all of the above plus some other permissions. Note that if you
prefer finer-grained privileges, you can just grant the ones described in
the Table. Also, you can refer to
[managing multiple projects](../install/managing-multiple-projects.md) in
case you want your Google Cloud Service Account to manage multiple
projects.

```shell
gcloud projects add-iam-policy-binding $PROJECT_ID \
--member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
--role roles/owner
```

## Option 1 (Recommended): Workload Identity

1. Enable Workload Identity.

    ```shell
    gcloud services enable iamcredentials.googleapis.com
    gcloud beta container clusters update $CLUSTER_NAME \
    --identity-namespace=$PROJECT_ID.svc.id.goog
    ```

1. Bind the Kubernetes Service Account `controller` with Google Cloud
    Service Account.

    ```shell
    MEMBER=serviceAccount:$PROJECT_ID.svc.id.goog[cloud-run-events/controller]

    gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member $MEMBER cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com
    ```

1. Add annotation to Kubernetes Service Account `controller`.

    ```shell
    kubectl annotate serviceaccount controller iam.gke.io/gcp-service-account=cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
    --namespace cloud-run-events
    ```

## Option 2: Kubernetes Secrets

1. Download a new JSON private key for that Service Account. **Be sure
    not to check this key into source control!**

    ```shell
    gcloud iam service-accounts keys create cloud-run-events.json \
    --iam-account=cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com
    ```

1. Create a Secret on the Kubernetes cluster in the `cloud-run-events`
    namespace with the downloaded key:

    ```shell
    kubectl --namespace cloud-run-events create secret generic google-cloud-key --from-file=key.json=cloud-run-events.json
    ```

    Note that `google-cloud-key` and `key.json` are default values
    expected by our control plane.

1. Restart controller with:

    ```shell
    kubectl delete pod -n cloud-run-events --selector role=controller
    ```
