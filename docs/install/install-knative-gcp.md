# Installing Knative-GCP

## Setup your environment

To start your environment you'll need to set these environment variables (we
recommend adding them to your `.bashrc`):

1. `GOPATH`: If you don't have one, simply pick a directory and add
   `export GOPATH=...`
1. `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.
1. `KO_DOCKER_REPO`: The docker repository to which developer images should be
   pushed (e.g. `gcr.io/[gcloud-project]`).

> :information_source: If you are using Docker Hub to store your images, your
> `KO_DOCKER_REPO` variable should have the format `docker.io/<username>`.
> Currently, Docker Hub doesn't let you create subdirs under your username (e.g.
> `<username>/knative`).

`.bashrc` example:

```shell
export GOPATH="$HOME/go"
export PATH="${PATH}:${GOPATH}/bin"
export KO_DOCKER_REPO='gcr.io/my-gcloud-project-id'
```

## Prerequisites

1. You must have [`ko`](https://github.com/google/ko) installed.

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This guide will use
   a mix of `gcloud` and `kubectl` commands. The rest of the guide assumes that
   you've set the `PROJECT_ID` environment variable to your Google Cloud project
   id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. Create a cluster under your Google Cloud project. If you would like to use
   [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
   to configure credential in the section **_Configure the Authentication
   Mechanism for GCP_**, we recommend you to enable Workload Identity when you
   create cluster, this could help to reduce subsequent configuration time.

1. Set up a Linux Container repository for pushing images. You can use any
   container image registry by adjusting the authentication methods and
   repository paths mentioned in the sections below.

   - [Google Container Registry quickstart](https://cloud.google.com/container-registry/docs/pushing-and-pulling)
   - [Docker Hub quickstart](https://docs.docker.com/docker-hub/)

   > :information_source: You'll need to be authenticated with your
   > `KO_DOCKER_REPO` before pushing images. Run `gcloud auth configure-docker`
   > if you are using Google Container Registry or `docker login` if you are
   > using Docker Hub.

1. Install [Knative](https://knative.dev/docs/install/). Set up
   [Serving](https://knative.dev/docs/serving/) and
   [Eventing](https://knative.dev/docs/eventing/). Both are required for now to
   make knative-gcp work.

1. Download the `knative-gcp` source code into `$GOPATH/src/github.com/google/knative-gcp`.
   For development follow the [fork instructions](https://github.com/google/knative-gcp/blob/master/DEVELOPMENT.md#checkout-your-fork).

## Install the Knative-GCP Constructs

Enter the `knative-gcp` directory before running the following commands.

### Option 1: Install from Master using [ko](http://github.com/google/ko)

```shell
ko apply -f ./config
```

### Option 2: Install a [release](https://github.com/google/knative-gcp/releases).

1. Pick a knative-gcp release version:

   ```shell
   export KGCP_VERSION=v0.15.0
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

## Configure the Authentication Mechanism for GCP (the Control Plane)

Currently, we support two methods: Workload Identity and Kubernetes Secret. The
configuration steps have been automated by the scripts below. If you wish to
configure the auth manually, refer to
[Manually Configure Authentication Mechanism for the Control Plane](./authentication-mechanisms-gcp.md/#authentication-mechanism-for-the-control-plane).

Before applying initialization scripts, make sure:

1. Your default zone is set to be the same as your current cluster. You may use
   `gcloud container clusters describe $CLUSTER_NAME` to get zone and apply
   `gcloud config set compute/zone $ZONE` to set it.
1. Your gcloud `CLI` are up to date. You may use `gcloud components update` to
   update it.

### Option 1 (Recommended): Use Workload Identity.

Workload Identity is the recommended way to access Google Cloud services from
within GKE due to its improved security properties and manageability. For more
information about Workload Identity, please see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

**_Note:_** If you install the Knative-GCP Constructs with v0.14.0 or older
releases, please use option 2.

Apply [init_control_plane_gke.sh](../../hack/init_control_plane_gke.sh):

```shell
./hack/init_control_plane_gke.sh
```

**_Note_**: If you didn't enable Workload Identity when you created your
cluster, this step may take a long time to finish.

Optional parameters available:

1. `CLUSTER_NAME`: an optional parameter to specify the cluster to use, default
   to `gcloud config get-value run/cluster`
1. `CLUSTER_LOCATION`: an optional parameter to specify the cluster location to
   use, default to `gcloud config get-value run/cluster_location`
1. `CLUSTER_LOCATION_TYPE`: an optional parameter to specify the cluster
   location type to use, default to `zonal`. CLUSTER_LOCATION_TYPE must be
   `zonal` or `regional`.
1. `PROJECT_ID`: an optional parameter to specify the project to use, default to
   `gcloud config get-value project`.

Here is an example specifying the parameters instead of using the default ones:

```shell
./hack/init_control_plane_gke.sh [CLUSTER_NAME] [CLUSTER_LOCATION] [CLUSTER_LOCATION_TYPE] [PROJECT_ID]
```

### Option 2: Export service account keys and store them as Kubernetes Secrets.

Apply [init_control_plane.sh](../../hack/init_control_plane.sh):

```shell
./hack/init_control_plane.sh
```

Optional parameters available:

1.  `PROJECT_ID`: an optional parameter to specify the project to use, default
    to `gcloud config get-value project`. If you want to specify the parameter
    `PROJECT_ID` instead of using the default one,

```shell
./hack/init_control_plane.sh [PROJECT_ID]
```
