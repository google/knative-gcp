# Knative-GCP with Deployment Manager

This document describes how to install Knative-GCP using [Deployment Manager](https://cloud.google.com/deployment-manager).

## Prerequisites

Install `gcloud` and have it associated with a project with the Deployment Manager API activated.

Note, if you want to activate APIs or manipulate the project's IAM policy (to add the newly 
created GSA), then you need to increase the permission of the service account that Deployment Manager
uses to `roles/owner`:

```shell
gcloud projects add-iam-policy-binding $(gcloud config get-value core/project) \
  --role=roles/owner \
  --member=serviceAccount:$(gcloud projects describe $(gcloud config get-value core/project) --format="value(projectNumber)")@cloudservices.gserviceaccount.com
```

## Installation

Due to the way DM works (specifically acquiring existing resources without updating them), 
the same configuration has to be installed and then updated three times.
Each time, the only thing that should change is the `stage` property:
- `1_clusterCreate`
- `2_acquireOperator`
- `3_installEventing`
- `4_initializeEventing`

Create the cluster and set up events in the cluster:

1. Customize anything you want in [`cluster-plus-events.yaml`](./cluster-plus-events.yaml).
   ```yaml
   - name: events
     type: events.py
     properties:
        # stage will change over time, but should start with 1_clusterCreate.
        stage: 1_clusterCreate
        # Should be set to true because the cluster is created in the same Deployment.
        # Would be false if the cluster's TypeProvider already exists.
        dependsOnTypeProvider: true
        # The name of the TypeProvider. Created in the same Deployment, so do not change this.
        typeProvider: $(ref.events-plus-cluster.clusterType)
        # Set to false if you don't want the Deployment to manipulate your project's IAM
        # permissions. See the prerequisites, if this is true, then Deployment Manager's
        # Service Account needs additional permissions.
        grantGSAsPermissionsOnProject: true
   ```
   See other options in [`events.py.schema`](./events.py.schema).
   
1. Create the deployment using [`cluster-plus-events.yaml`](./cluster-plus-events.yaml).
   ```shell
   gcloud deployment-manager deployments create ce --config cluster-plus-events.yaml
   ```

1. Change the `stage` in `cluster-plus-events.yaml` to `2_acquireOperator`.
   ```yaml
   stage: 2_acquireOperator
   ```
   
1. Update the Deployment with the updated config.
   ```shell
   gcloud deployment-manager deployments udpate ce --config cluster-plus-events.yaml
   ```
   
1. Change the `stage` in `cluster-plus-events.yaml` to `3_installEventing`.
   ```yaml
   stage: 3_installEventing
   ```
   
1. Update the Deployment with the updated config.
   ```shell
   gcloud deployment-manager deployments udpate ce --config cluster-plus-events.yaml
   ```

1. Change the `stage` in `cluster-plus-events.yaml` to `4_initializeEventing`.
   ```yaml
   stage: 4_initializeEventing
   ```

1. Update the Deployment with the updated config.
   ```shell
   gcloud deployment-manager deployments udpate ce --config cluster-plus-events.yaml
   ```

1. Populate `kubectl` with the cluster's information.
   ```shell
   gcloud container clusters get-credentials --zone us-central1-a ce-events-plus-cluster
   ```

## Usage

You can deploy:
- `Broker`
- `Trigger`
- `CloudPubSubSource`
- `Knative Service` - ksvc
- `ConfigMap`

### Examples

- Broker, Trigger, CloudPubSubSource, and KSVC: [`broker.yaml`](./broker.yaml).
- KSVC and ConfigMap: [`ksvc.yaml`](./ksvc.yaml).
