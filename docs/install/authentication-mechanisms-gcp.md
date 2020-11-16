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

This is the manual auth configuration for the Data Plane and
using the Google Cloud Service Account `cre-dataplane` as the credential.
Refer to [Installing a Service Account for the Data Plane](../install/dataplane-service-account.md) for automated scripts.

### Option 1: Use Workload Identity
1. We assume that you have already [enabled Workload Identity](../install/authentication-mechanisms-gcp.md/#option-1-recommended-workload-identity)
in your cluster.
1. There are two scenarios to leverage Workload Identity for resources in the Data
Plane:

- **_Non-default scenario:_**

  Using the Google Cloud Service Account `cre-dataplane` you created in [Installing a Service Account for the Data Plane](../install/dataplane-service-account.md)
  and using
  [Option 1 (Recommended): Workload Identity](../install/authentication-mechanisms-gcp.md/#option-1-recommended-workload-identity)
  in [Authentication Mechanism for GCP](../install/authentication-mechanisms-gcp.md)
  to configure Workload Identity in the namespace your resources will reside.
  (You may notice that this link is pointing to the manual Workload Identity
  configuration in the Control Plane. Non-default scenario Workload Identity
  configuration in the Data Plane is similar to the manual Workload Identity
  configuration in the Control Plane)

  You will have a Kubernetes Service Account after the above configuration,
  which is bound to the Google Cloud Service Account `cre-dataplane`. Remember
  to put this Kubernetes Service Account name as the `spec.serviceAccountName`
  when you create resources in the
  [example](https://github.com/google/knative-gcp/tree/master/docs/examples).

- **_Default scenario:_**

  Instead of manually configuring Workload Identity with
  [Authentication Mechanism for GCP](../install/authentication-mechanisms-gcp.md),
  you can authorize the Controller to configure Workload Identity for you.

  You need to grant `iam.serviceAccountAdmin` permission of the Google Cloud
  Service Account `cre-dataplane` you created in [Installing a Service Account for the Data Plane](../install/dataplane-service-account.md)
  to the Control Plane's Google Cloud Service Account `cloud-run-events` by:

  ```shell
  gcloud iam service-accounts add-iam-policy-binding \
   cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com  \
   --member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
   --role roles/iam.serviceAccountAdmin
  ```

  Then, modify `clusterDefaults` in ConfigMap `config-gcp-auth`.

  You can directly edit the ConfigMap by:

  ```shell
  kubectl edit configmap config-gcp-auth -n cloud-run-events
  ```

  and add `workloadIdentityMapping` in `clusterDefaults`:

  ```shell
  default-auth-config: |
    clusterDefaults:
      workloadIdentityMapping:
        serviceAccountName: default-cre-dataplane
        default-cre-dataplane: cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com
  ```

  When updating the configuration, note that `default-auth-config` is nested
  under `data`. If you encounter an error, you are likely attempting to modify
  the example configuration in `_example`.

  Here, `default-cre-dataplane` refers to a Kubernetes Service Account bound to
  the Google Cloud Service Account `cre-dataplane`. Remember to put this
  Kubernetes Service Account name as the `spec.serviceAccountName` when you
  create resources in the
  [example](https://github.com/google/knative-gcp/tree/master/docs/examples).

  Kubernetes Service Account `default-cre-dataplane` doesn't need to exist in a
  specific namespace. Once it is set in the ConfigMap `config-gcp-auth`, the
  Control Plane will create it for you and configure the corresponding Workload
  Identity relationship between the Kubernetes Service Account
  `default-cre-dataplane` and the Google Cloud Service Account `cre-dataplane`
  when you create resources using the Kubernetes Service Account
  `default-cre-dataplane`.

  A `Condition` `WorkloadIdentityConfigured` will show up under resources'
  `Status`, indicating the Workload Identity configuration status.

 **_Note:_** The Controller currently doesn’t perform any access control checks,
as a result, any user who can create a resource can get access to the Google Cloud
Service Account which grants the `iam.serviceAccountAdmin` permission to the Controller.
As an example, if you followed the instructions above, then any user that can make
a Knative-GCP source or Channel (e.g. `CloudAuditLogsSource`, `CloudPubSubSource`,
etc.) can cause the Kubernetes Service Account `default-cre-dataplane` to be created.
If they can also create Pods in that namespace, then they can make a Pod that uses
the Google Service Account `cre-dataplane` credentials.

### Option 2. Export Service Account Keys And Store Them as Kubernetes Secrets

1. Download a new JSON private key for the Google Cloud Service Account `cre-dataplane`
created in [Installing a Service Account for the Data Plane](../install/dataplane-service-account.md).
 **Be sure not to check this key into source control!**

   ```shell
   gcloud iam service-accounts keys create cre-dataplane.json \
   --iam-account=cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com
   ```

1. Create a secret on the Kubernetes cluster with the downloaded key. Remember
   to create the secret in the namespace your resources will reside. The example
   below does so in the `default` namespace.

   ```shell
   kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=cre-dataplane.json
   ```

   `google-cloud-key` and `key.json` are default values expected by our
   resources.
1. Cleaning Up:
   1. Delete the secret

       ```shell
       kubectl --namespace default delete secret google-cloud-key
       ```
