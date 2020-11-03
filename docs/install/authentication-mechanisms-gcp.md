# Configure Authentication Mechanism for GCP

## Authentication Mechanism for the Control Plane

This is the manual auth configuration for the Control Plane. Refer to the
[installation guide](./install-knative-gcp.md) for automated scripts.

For both methods (Workload Identity and Kubernetes Secret), the first manual
step is creating a
[Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
with the appropriate permissions needed for the control plane to manage GCP
resources.

You need to create a new Google Cloud Service Account named `cloud-run-events`
with the following command:

```shell
gcloud iam service-accounts create cloud-run-events
```

Then, give that Google Cloud Service Account permissions on your project. The
actual permissions needed will depend on the resources you are planning to use.
The Table below enumerates such permissions:

| Resource / Functionality |                                     Roles                                      |
| :----------------------: | :----------------------------------------------------------------------------: |
|    CloudPubSubSource     |                              roles/pubsub.editor                               |
|    CloudStorageSource    |                              roles/storage.admin                               |
|   CloudSchedulerSource   |                           roles/cloudscheduler.admin                           |
|   CloudAuditLogsSource   | roles/pubsub.admin, roles/logging.configWriter, roles/logging.privateLogViewer |
|     CloudBuildSource     |                            roles/pubsub.subscriber                             |
|         Channel          |                              roles/pubsub.editor                               |
|     PullSubscription     |                              roles/pubsub.editor                               |
|          Topic           |                              roles/pubsub.editor                               |

In this guide, and for the sake of simplicity, we will just grant `roles/owner`
privileges to the Google Cloud Service Account, which encompasses all of the
above plus some other permissions. Note that if you prefer finer-grained
privileges, you can just grant the ones described in the Table. Also, you can
refer to [managing multiple projects](../install/managing-multiple-projects.md)
in case you want your Google Cloud Service Account to manage multiple projects.

```shell
gcloud projects add-iam-policy-binding $PROJECT_ID \
--member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
--role roles/owner
```

### Option 1 (Recommended): Workload Identity

1. Enable Workload Identity.

   1. Ensure that you have enabled the Cloud IAM Service Account Credentials
      API.
      ```shell
      gcloud services enable iamcredentials.googleapis.com
      ```
   1. If you didn't enable Workload Identity when you created your cluster, run
      the following commands:

      Modify the cluster to enable Workload Identity first:

      ```shell
      gcloud container clusters update $CLUSTER_NAME \
        --workload-pool=$PROJECT_ID.svc.id.goog
      ```

      Enable `GKE_METADATA` for your node pool(s).

      ```shell
       pools=$(gcloud container node-pools list --cluster=${CLUSTER_NAME} --format="value(name)")
       while read -r pool_name
       do
         gcloud container node-pools update "${pool_name}" \
           --cluster=${CLUSTER_NAME} \
           --workload-metadata=GKE_METADATA
       done <<<"${pools}"
      ```

      **_NOTE_**: These commands may take a long time to finish. Check
      [Enable Workload Identity on an existing cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#enable_on_an_existing_cluster)
      for more information.

1. Create a Kubernetes Service Account `ksa-name` in the namespace your
   resources will reside. If you are configuring Authentication Mechanism for
   the Control Plane, you can skip this step, and directly use Kubernetes
   Service Account `controller` which is already in the Control Plane namespace
   `cloud-run-events`

   ```shell
   kubectl create serviceaccount ksa-name -n ksa-namespace
   ```

1. Bind the Kubernetes Service Account `ksa-name` with Google Cloud Service
   Account.

   ```shell
   MEMBER=serviceAccount:$PROJECT_ID.svc.id.goog[ksa-namespace/ksa-name]

   gcloud iam service-accounts add-iam-policy-binding \
   --role roles/iam.workloadIdentityUser \
   --member $MEMBER gsa-name@$PROJECT_ID.iam.gserviceaccount.com
   ```

   If you are configuring Authentication Mechanism for the Control Plane, you
   can replace `ksa-namespace` with `cloud-run-events`, `ksa-name` with
   `controller`, and `gsa-name` with `cloud-run-events`

1. Annotate the Kubernetes Service Account `ksa-name`.

   ```shell
   kubectl annotate serviceaccount ksa-name iam.gke.io/gcp-service-account=gsa-name@$PROJECT_ID.iam.gserviceaccount.com \
   --namespace ksa-namespace
   ```

   If you are configuring Authentication Mechanism for the Control Plane, you
   can replace `ksa-namespace` with `cloud-run-events`, `ksa-name` with
   `controller`, and `gsa-name` with `cloud-run-events`

### Option 2: Kubernetes Secrets

1. Download a new JSON private key for that Service Account. **Be sure not to
   check this key into source control!**

   ```shell
   gcloud iam service-accounts keys create cloud-run-events.json \
   --iam-account=cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com
   ```

1. Create a Secret on the Kubernetes cluster in the `cloud-run-events` namespace
   with the downloaded key:

   ```shell
   kubectl --namespace cloud-run-events create secret generic google-cloud-key --from-file=key.json=cloud-run-events.json
   ```

   Note that `google-cloud-key` and `key.json` are default values expected by
   our control plane.

## Authentication Mechanism for the Data Plane

Please check
[Installing a Service Account for the Data Plane](../install/dataplane-service-account.md).
