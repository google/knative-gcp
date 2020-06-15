# Configure Authentication Mechanism for GCP

## Authentication Mechanism for the Control Plane
For both methods (Workload Identity and Kubernetes Secret), the first manual
step is creating a
[Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
with the appropriate permissions needed for the control plane to manage native
GCP resources.

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
 
Please check [Installing Pub/Sub Enabled Service Account](../install/pubsub-service-account.md).
 
## Troubleshooting

### Workload Identity

To ensure Workload Identity correctly configured for a Google Cloud Service Account (GSA), 
and a Kubernetes Service Account (KSA), you need to check:

1. If the Google Cloud Service Account has the desired `iam-policy-binding`.
   
   By running the following command: 
   ```shell
   gcloud iam service-accounts get-iam-policy gsa-name@$PROJECT_ID.iam.gserviceaccount.com
   ```
   if the `iam-policy-binding` is correct, you'll find a `iam-policy-binding` similar to:
   ```shell
    bindings:
    - members:
      - serviceAccount:$PROJECT_ID.svc.id.goog[ksa-namespace/ksa-name]
      role: roles/iam.workloadIdentityUser
   ```
 2. If the Kubernetes Service Account has the desired annotation.
   
    By running the following command: 
    ```shell
    kubectl get serviceaccount ksa-name -n ksa-namespace -o yaml 
    ```
    if the annotation exists, you'll find an annotation similar to:
    ```shell
    metadata:
      annotations:
        iam.gke.io/gcp-service-account: gsa-name@$PROJECT_ID.iam.gserviceaccount.com
    ```

### Kubernetes Secrets

To ensure kubernetes Secret correctly configured in a namespace, you need to check if a Kubernetes Secret 
with the correct name is in the namespace. 

By default, this secret is named `google-cloud-key`, but it could
be set to something else either in the custom object itself (e.g. the Source's `spec.secret`) or 
in the `config-gcp-auth` ConfigMap. Verify that a secret with the correct name is in the same namespace 
as the custom object by running the following command:

```shell
kubectl get secret -n namespace
```

### Common Issues

* ***Resources are not READY***

   If a resource instance is not READY due to authentication configuration problem, 
   you are likely to get `PermissionDenied` error for `User not authorized to perform this action`
   (for Workload Identity, the error might look like `IAM returned 403 Forbidden: The caller does not have permission`).
   
   This error indicates that the Control Plane may not configure authentication properly. 
   You can find it in a resource instance by:
   
   ```shell
   kubectl describe resource-name resource-instance-name -n resource-instance-namespace
   ```
   
   Here is an example checking a CloudAuditlogsSource `test` in namespace `default`:
      
   ```shell
   kubectl describe cloudauditlogssource test -n default
   ```
   
   To solve this issue, you can:
   - Check the Google Cloud Service Account `cloud-run-events` for the Control Plane has all required permissions.
   - Check authentication configuration is correct for the Control Plane. 
     - If you are using Workload Identity for the Control Plane, 
     refer [here](./authentication-mechanisms-gcp.md/#workload-identity) to check the
     Google Cloud Service Account `cloud-run-events`, 
     and the Kubernetes Service Account `controller` in namespace `cloud-run-events`.
     - If you are using Kubernetes Secret for the Control Plane, 
     refer [here](./authentication-mechanisms-gcp.md/#kubernetes-secrets) to check the 
     Kubernetes Secret `google-cloud-key` in namespace `cloud-run-events`.
     
   ***Note:*** For Kubernetes Secret, it is also possible that the JSON private key is no longer existing under your Google Cloud Service Account 
   `cloud-run-events`. Then, even the Google Cloud Service Account `cloud-run-events` has all required permissions, and 
   the corresponding Kubernetes Secret `google-cloud-key ` is in namespace `cloud-run-events`, you still get `PermissionDenied` error. 
   To such case, you have to re-download the JSON private key and re-create the Kubernetes Secret, refer 
   [here](./authentication-mechanisms-gcp.md/#option-2-kubernetes-secrets) for instructions.
     
* ***Resources are READY, but can't receive Events***

   Sometimes, a resource instance is READY, but it can't receive Events. It might be an authentication configuration problem, 
   if the underlying `Deployment` doesn't have minimum availability.
    
   Typically, the name of an 
   underlying `Deployment` for a resource instance could be a truncated version of 
   `(prefix)-(resource-instance-name)-(uid)`. 
   1. If the resource instance is a `Source`, the prefix is `cre-src`. 
   1. If the resource instance is a `Channel`, the prefix is `cre-chan`. 
   1. If the resource instance is a `Pullsubscription`, the prefix is `cre-ps`
   
   You can use the following command to check if the underlying `Deployment`'s available pod is zero.
   
   ```shell
   kubectl get deployment -n resource-instance-namespace
   ```
   Here is an example checking a CloudAuditlogsSource `test` in namespace `default`:
     
   ```shell
   kubectl describe cloudauditlogssource test -n default
   ```
      
   ***To solve this issue***, you can:
   - Check the Google Cloud Service Account `cre-pubsub` for the Data Plane has all required permissions.
   - Check authentication configuration is correct for this resource instance. 
     - If you are using Workload Identity for this resource instance, 
     refer [here](./authentication-mechanisms-gcp.md/#workload-identity) to check the
     Google Cloud Service Account `cre-pubsub`, 
     and the Kubernetes Service Account in the namespace where this resource instance resides.
     - If you are using Kubernetes Secret for this resource instance, 
     refer [here](./authentication-mechanisms-gcp.md/#kubernetes-secrets) to check the 
     Kubernetes Secret in namespace where this resource instance resides.
     
   ***Note:*** For Workload Identity, there is a known issue [#759](https://github.com/google/knative-gcp/issues/759) 
   for credential sync delay (~1 min) in resources' underlying components. You'll probably encounter this issue, 
   if you immediately send Events after you finish Workload Identity setup for a resource instance.
   
* ***Resources are not READY, due to WorkloadIdentityReconcileFailed***
   
   This error only exists when you use default scenario for Workload Identity. 
   You can find detail failure reason by:
   ```shell
   kubectl describe resource-name resource-instance-name -n resource-instance-namespace
   ```
   If the reason related on permission denied, refer to [Common Issues](./authentication-mechanisms-gcp.md/#common-issues) ***Resources are not READY*** for solution.
   
   If the reason related on concurrency issue, the controller will retry it in the next reconciliation loop 
   (the maximum retry period is 5 min). You can also use [non-default scenario](../install/pubsub-service-account.md) 
   if this error least for a long time.
   