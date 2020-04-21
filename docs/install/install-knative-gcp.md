# Installing Knative-GCP

## Prerequisites

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This guide will use
   a mix of `gcloud` and `kubectl` commands. The rest of the guide assumes that
   you've set the `PROJECT_ID` environment variable to your Google Cloud project
   id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. Install [Knative](https://knative.dev/docs/install/). Preferably, set up both
   [Serving](https://knative.dev/docs/serving/) and
   [Eventing](https://knative.dev/docs/eventing/). The latter is only required
   if you want to use the Pub/Sub `Channel` or a `Broker` backed by a Pub/Sub
   `Channel`.

## Install the Knative-GCP Constructs

### Option 1: Install from Master using [ko](http://github.com/google/ko)

```shell
ko apply -f ./config
```

### Option 2: Install a [release](https://github.com/google/knative-gcp/releases).

1. Pick a knative-gcp release version:

   ```shell
   export KGCP_VERSION=v0.14.0
   ```

1. First install the CRDs by running the `kubectl apply` command with the
   `--selector` flag. This prevents race conditions during the install, which
   cause intermittent errors:

   ```shell
   kubectl apply --selector messaging.cloud.google.com/crd-install=true \
   --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   kubectl apply --selector events.cloud.google.com/crd-install=true \
   --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   ```

1. To complete the install run the `kubectl apply` command again, this time
   without the `--selector` flags:

   ```shell
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   ```

## Configure the Authentication Mechanism for GCP

Currently, we support two methods: Workload Identity and Kubernetes Secret.
Workload Identity is the recommended way to access Google Cloud services from
within GKE due to its improved security properties and manageability. For more
information about Workload Identity, please see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

**_Note_**: Before applying initialization scripts, make sure your default zone
is set to be the same as your current cluster. You may use
`gcloud container clusters describe $CLUSTER_NAME` to get zone and apply
`gcloud config set compute/zone $ZONE` to set it.

**_Note_**: Both scripts will have a step to create a Google Cloud Service
Account `cloud-run-events`. Ignore the error message if you already had this
service account (error for 'service account already exists').
TODO([#896](https://github.com/google/knative-gcp/issues/896)) Get rid of the
error message.

**_Note_**: The configuration steps have been automated by the scripts below. If
wish to configure the auth manually, refer to
[manually configure authentication for GCP](./authentication-mechanisms-gcp.md),

- Option 1 (Recommended): Use Workload Identity. Apply
  [init_control_plane_gke.sh](../../hack/init_control_plane_gke.sh):

  ```shell
  ./hack/init_control_plane_gke.sh
  ```

- Option 2: Export service account keys and store them as Kubernetes Secrets.
  Apply [init_control_plane.sh](../../hack/init_control_plane.sh):

  ```shell
  ./hack/init_control_plane.sh
  ```
