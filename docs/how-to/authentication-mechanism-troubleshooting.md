# Authentication Mechanism Troubleshooting

## Workload Identity

To ensure Workload Identity is correctly configured for a Google Cloud Service
Account (GSA), and a Kubernetes Service Account (KSA), you need to check:

1. If the Google Cloud Service Account has the desired `iam-policy-binding`.

   By running the following command:

   ```shell
   gcloud iam service-accounts get-iam-policy gsa-name@$PROJECT_ID.iam.gserviceaccount.com
   ```

   if the `iam-policy-binding` is correct, you'll find an `iam-policy-binding`
   similar to:

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

## Kubernetes Secrets

To ensure kubernetes Secrets correctly are configured in a namespace, you need
to check if a Kubernetes Secret with the correct name is in the namespace.

By default, this secret is named `google-cloud-key`, but it could be set to
something else either in the custom object itself (e.g. the Source's
`spec.secret`) or in the `config-gcp-auth` ConfigMap. Verify that a secret with
the correct name is in the same namespace as the custom object by running the
following command:

```shell
kubectl get secret -n namespace
```

## Common Issues

### Resources are not READY

If a resource instance is not READY due to an authentication configuration
problem, you are likely to get permission related error for
`User not authorized to perform this action` (for Workload Identity, the error
might look like
`IAM returned 403 Forbidden: The caller does not have permission`).

This error indicates that the Control Plane may not configure authentication
properly. You can find it in a resource instance by:

```shell
kubectl describe resource-kind resource-instance-name -n resource-instance-namespace
```

---

Here is a detailed example checking a CloudAuditlogsSource `test` in namespace
`default`:

After running the following command, you will find a couple of `Condition`s
under this CloudAuditLogsSource's `Status`.

```shell
kubectl describe cloudauditlogssource test -n default
```

- If this CloudAuditLogsSource failed due to a missing Kubernetes Secret (for
  Kubernetes Secret authentication configuration), or a missing Kubernetes
  Service Account annotation (for Workload Identity authentication
  configuration), the `Condition` `Ready` would look like:

  ```shell
  Type:     Ready
  Status:   False
  Message:  Failed to reconcile Pub/Sub topic: rpc error: code = PermissionDenied desc = User not authorized to perform this action.
  Reason:   TopicReconcileFailed
  ```

- If this CloudAuditLogsSource failed due to the JSON private key stored in the
  Kubernetes Secret having been revoked (for Kubernetes Secret authentication
  configuration), the `Condition` `Ready` would look like:

  ```shell
  Type:     Ready
  Status:   False
  Message:  Failed to reconcile Pub/Sub topic: rpc error: code = Unauthenticated desc = transport: oauth2: cannot fetch token: 400 Bad Request
            Response: {"error":"invalid_grant","error_description":"Invalid JWT Signature."}
  Reason:   TopicReconcileFailed
  ```

- If this CloudAuditLogsSource failed due to the GSA having insufficient
  permissions (for either Kubernetes Secret authentication configuration or
  Workload Identity authentication configuration), the `Condition` `Ready` would
  look like:

  ```shell
  Type:     Ready
  Status:   False
  Message:  Failed to ensure creation of logging sink: rpc error: code = PermissionDenied desc = The caller does not have permission
  Reason:   SinkCreateFailed
  ```

- If this CloudAuditLogsSource failed due to the `iam-policy-binding` being
  setup incorrectly on the Google Service Account (for Workload Identity
  authentication configuration), the `Condition` `Ready` would look like:
  ```shell
  Type:     Ready
  Status:   False
  Message:  Failed to reconcile Pub/Sub topic: rpc error: code = Unauthenticated desc = transport: compute: Received 403 `
            unable to generate token; IAM returned 403 Forbidden: The caller does not have permission
            his error could be caused by a missing IAM policy binding on the target IAM service account.
  Reason:   TopicReconcileFailed
  ```

**_To solve this issue_**, you can:

- Check the Google Cloud Service Account `cloud-run-events` for the Control
  Plane has all required permissions.
- Check authentication configuration is correct for the Control Plane.

  - If you are using Workload Identity for the Control Plane, refer
    [here](../install/authentication-mechanisms-gcp.md/#workload-identity) to
    check the Google Cloud Service Account `cloud-run-events`, and the
    Kubernetes Service Account `controller` in namespace `cloud-run-events`.
  - If you are using Kubernetes Secret for the Control Plane, refer
    [here](../install/authentication-mechanisms-gcp.md/#kubernetes-secrets) to
    check the Kubernetes Secret `google-cloud-key` in namespace
    `cloud-run-events`.

**_Note:_** For Kubernetes Secret, if the JSON private key no longer exists
under your Google Cloud Service Account `cloud-run-events`. Then, even the
Google Cloud Service Account `cloud-run-events` has all required permissions,
and the corresponding Kubernetes Secret `google-cloud-key` is in namespace
`cloud-run-events`, you still get permission related error. To such case, you
have to re-download the JSON private key and re-create the Kubernetes Secret,
refer
[here](../install/authentication-mechanisms-gcp.md/#option-2-kubernetes-secrets)
for instructions.

### Resources are READY, but can't receive Events

Sometimes, a resource instance is READY, but it can't receive Events. It might
be an authentication configuration problem, if the underlying `Deployment`
doesn't have minimum availability.

Typically, the name of an underlying `Deployment` for a resource instance could
be a truncated version of `(prefix)-(resource-instance-name)-(uid)`.

1. If the resource instance is a `Source`, the prefix is `cre-src`.
1. If the resource instance is a `Channel`, the prefix is `cre-chan`.
1. If the resource instance is a `Pullsubscription`, the prefix is `cre-ps`

You can use the following command to check if the underlying `Deployment`'s
available pod is zero.

```shell
kubectl get deployment -n resource-instance-namespace
```

---

Here is a detailed example checking a CloudAuditlogsSource `test`'s underlying
`Deployment` in namespace `default`:

After running the following command, you will find a `Deployment` named as
`cre-src-cloudauditlogssource-te(UID)`.

```shell
kubectl get deployment -n default
```

- If this `Deployment` doesn't have minimum availability due to a missing
  Kubernetes Secret (for Kubernetes Secret authentication configuration), the
  `Pod` which belongs to this `Deployment` (`Pod`'s name will have the same
  prefix `cre-src-cloudauditlogssource-` as the `Deployment`'s name) will block
  at `ContainerCreating` status. Using
  `kubectl describe pod pod-name -n namespace`, you can find a Warning Event
  like this:

  ```shell
  Warning  FailedMount  27s (x2 over 2m40s)   kubelet, gke-knative-1-default-pool-73d30583-c1hv
  Unable to mount volumes for pod "cre-src-cloudauditlogssource-tef1a55bfca8e1d5620e0c4199495mwk6k_default(f35883bd-ee66-4497-94fa-05196a6043fe)":
  timeout expired waiting for volumes to attach or mount for pod "default"/"cre-src-cloudauditlogssource-tef1a55bfca8e1d5620e0c4199495mwk6k".
  list of unmounted volumes=[google-cloud-key]. list of unattached volumes=[google-cloud-key default-token-dd9cd]
  ```

- If this `Deployment` doesn't have minimum availability due to the JSON private
  key stored in the Kubernetes Secret having been revoked (for Kubernetes Secret
  authentication configuration), the `Pod` which belongs to this `Deployment`
  (`Pod`'s name will have the same prefix `cre-src-cloudauditlogssource-` as the
  `Deployment`'s name) will block at `Error` status. Using
  `kubectl log pod-name -n namespace`, you can find a Log containing information
  like this:

  ```shell
  "msg":"failed to start adapter: ",
  "commit":"9e8388f",
  "error":"unable to create subscription \"cre-src_default_cloudauditlogssource-test_a2ca50b7-c951-4682-b8f0-e902a449dbb1\",
  rpc error: code = Unauthenticated desc = transport: oauth2: cannot fetch token: 400 Bad Request\nResponse: {\"error\":\"invalid_grant\",\"error_description\":\"Invalid JWT Signature.\"}"
  ```

- If this `Deployment` doesn't have minimum availability due to the GSA having
  insufficient permissions (for either Kubernetes Secret authentication
  configuration or Workload Identity authentication configuration), the `Pod`
  which belongs to this `Deployment` (`Pod`'s name will have the same prefix
  `cre-src-cloudauditlogssource-` as the `Deployment`'s name) will block at
  `Error` status. Using `kubectl log pod-name -n namespace`, you can find a Log
  containing information like this:

  ```shell
  "msg":"failed to start adapter: ",
  "commit":"9e8388f",
  "error":"unable to create subscription \"cre-src_default_cloudauditlogssource-test_663a7f95-8b82-46aa-85b8-8fee4e65b26f\",
  rpc error: code = PermissionDenied desc = User not authorized to perform this action.
  ```

- If this `Deployment` doesn't have minimum availability due to a missing
  Kubernetes Service Account (for Workload Identity authentication
  configuration), the `Deployment` can't create any `Pod`. Using
  `kubectl describe deployment-name -n namespace`, you can find a `Condition`
  `ReplicaFailure` under `Status` like this:

  ```shell
  type: ReplicaFailure
  status: "True"
  reason: FailedCreate
  message: 'pods "cre-src-cloudauditlogssource-tebc146e4349a11efaf078c20f8ab65089-6b6b57997d-"
    is forbidden: error looking up service account default/test1: serviceaccount
    "test1" not found'
  ```

- If this `Deployment` doesn't have minimum availability due to a missing
  Kubernetes Service Account annotation (for Workload Identity authentication
  configuration), the `Pod` which belongs to this `Deployment` (`Pod`'s name
  will have the same prefix `cre-src-cloudauditlogssource-` as the
  `Deployment`'s name) will block at `Error` status. Using
  `kubectl log pod-name -n namespace`, you can find a Log containing information
  like this:

  ```shell
  "msg":"failed to start adapter: ",
  "commit":"9e8388f",
  "error":"unable to create subscription \"cre-src_default_cloudauditlogssource-test_663a7f95-8b82-46aa-85b8-8fee4e65b26f\",
  rpc error: code = PermissionDenied desc = User not authorized to perform this action.
  ```

- If this `Deployment` doesn't have minimum availability due to the
  `iam-policy-binding` being setup incorrectly (for Workload Identity
  authentication configuration), the `Pod` which belongs to this `Deployment`
  (`Pod`'s name will have the same prefix `cre-src-cloudauditlogssource-` as the
  `Deployment`'s name) will block at `Error` status. Using
  `kubectl log pod-name -n namespace`, you can find a Log containing information
  like this:
  ```shell
  "msg":"failed to start adapter: ",
  "commit":"9e8388f",
  "error":"unable to create subscription \"cre-src_default_cloudauditlogssource-test_4daeacfe-6155-4f69-8d52-64745fa0591d\",
  rpc error: code = Unauthenticated desc = transport: compute: Received 403 `\nUnable to generate token; IAM returned 403 Forbidden:
  The caller does not have permission\n\nThis error could be caused by a missing IAM policy binding on the target IAM service account.
  ```
  ***
  **_To solve this issue_**, you can:

* Check the Google Cloud Service Account `cre-dataplane` for the Data Plane has
  all required permissions.
* Check authentication configuration is correct for this resource instance.

  - If you are using Workload Identity for this resource instance, refer
    [here](../install/authentication-mechanisms-gcp.md/#workload-identity) to
    check the Google Cloud Service Account `cre-dataplane`, and the Kubernetes
    Service Account in the namespace where this resource instance resides.
  - If you are using Kubernetes Secrets for this resource instance, refer
    [here](../install/authentication-mechanisms-gcp.md/#kubernetes-secrets) to
    check the Kubernetes Secret in namespace where this resource instance
    resides.

**_Note:_** For Workload Identity, there is a known issue
[#759](https://github.com/google/knative-gcp/issues/759) for credential sync
delay (~1 min) in resources' underlying components. You'll probably encounter
this issue, if you immediately send Events after you finish Workload Identity
setup for a resource instance.

### Resources are not READY, due to WorkloadIdentityReconcileFailed

This error only exists when you use default scenario for Workload Identity. You
can find detailed failure message by:

```shell
kubectl describe resource-kind resource-instance-name -n resource-instance-namespace
```

**_To solve this issue_**, you can:

- Make sure the `workloadIdentityMapping` under `default-auth-config` in
  `ConfigMap` `config-gcp-auth` is correct (a correct Kubernetes Service Account
  paired with a correct Google Cloud Service Account).
- If the `Condition` `Ready` has permission related error message like this:
  ```shell
  type: Ready
  status: "False"
  message: 'rpc error: code = PermissionDenied desc = Permission iam.serviceAccounts.setIamPolicy
    is required to perform this operation on service account projects/-/serviceAccounts/cre-dataplane@PROJECT_ID.iam.gserviceaccount.com.'
  reason: WorkloadIdentityFailed
  ```
  it is most likely that you didn't grant `iam.serviceAccountAdmin` permission
  of the Google Cloud Service Account to the Control Plane's Google Cloud
  Service Account `cloud-run-events`, refer to
  [default scenario](../install/dataplane-service-account.md/#option-1-use-workload-identity)
  to grant permission.
- If the `Condition` `Ready` has concurrency related error message like this:
  ```shell
  type: Ready
  status: "False"
  message: 'adding iam policy binding failed with: failed to set iam policy: googleapi: Error 409: There were concurrent policy changes.
    Please retry the whole read-modify-write with exponential backoff'
  reason: WorkloadIdentityFailed
  ```
  the controller will retry it in the next reconciliation loop (the maximum
  retry period is 5 min). You can also use
  [non-default scenario](../install/dataplane-service-account.md/#option-1-use-workload-identity)
  if this error lasts for a long time.
