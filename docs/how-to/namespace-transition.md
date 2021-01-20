# Upgrade cluster from eventing release v0.19 to v0.20

**This documentation is for clusters that already have release `v0.19` of
eventing running in the `cloud-run-events` namespace. If you are installing
eventing for the first time on your cluster, you do not need to follow this
steps and can go directly to
[Installing Knative-GCP](https://github.com/google/knative-gcp/blob/master/docs/install/install-knative-gcp.md).**

The upcoming `v0.20` release of `knative-gcp` will contain changes that are not
compatible with old releases. Namely, the namespace will change from
`cloud-run-events` to `events-system`. Using the new release without running the
migration steps described below will result in downtime and errors produced by
both the previous and current installation.

To avoid these issue we recommend taking the proactive approach described in
[Recommended Migration](#recommended-migration) to ensure continued good
functionality of your installed events system.

**Note that the current instructions only cover installation based on the
release from the `knative-gcp` repository. We will update the page with specific
`gcloud` instructions as the release becomes available there as well.**

## Recommended Migration

Migrating your events resources to the new namespace involves two main steps:

1. Upgrading to the new version of events and setting up authentication for the
   new namespace.
2. Verifying the correct functionality of already deployed resources.

We assume here that the desire is to keep the existing authentication mechanism
and provide the steps to correct the permissions to work in the new namespace.
For this purpose we need to first setup a number of environment variables based
on the existing authentication settings. The following `gcloud` command can be
used to view the existing Google Service Accounts (GSAs) and their roles.

```shell script
# view GSAs and roles
gcloud projects get-iam-policy $PROJECT_ID
```

Using this information, you can find the service accounts set up for the control
plane and broker data plane. The control plane GSA should have the roles:
`pubsub.admin`, `storage.admin`, `cloudscheduler.admin`, `logging.configWriter`
and `logging.privateLogViewer`. The broker data plane GSA should have the roles:
`pubsub.editor`, `monitoring.metricWriter` and `cloudtrace.agent`.

Once you found the names for these service accounts, set the following
environment variables. In this example the names of the GSAs are
`events-controller-gsa` for the control plane and `events-broker-gsa` for the
broker data plane.

```
export CONTROLLER_GSA=events-controller-gsa
export BROKER_GSA=events-broker-gsa
```

Next follow the upgrade instructions for authentication with either Kubernetes
Secrets or Workload Identity, depending on what was used in the old namespace.
After you've completed the upgrade verify that everything is working correctly
by following the steps in the last section.

### Upgrade for Authentication using Kubernetes Secrets

1. Run the following commands before installing the new `v0.20` release. We
   explain the purpose of each command in comments.

   ```shell script
   # create the new namespace by copying the labels and annotations from the old one
   kubectl get namespace cloud-run-events -o yaml | \
     sed 's/  name: cloud-run-events/  name: events-system/g' | \
     sed 's/\/cloud-run-events/\/events-system/g' | \
     kubectl apply -f -
   # copy the existing config maps to maintain existing configurations
   kubectl get configmaps -n cloud-run-events --field-selector metadata.name!=kube-root-ca.crt -o yaml | \
     sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
     sed 's/\/cloud-run-events\//\/events-system\//g' | \
     kubectl apply -f -

   # setup authentication for the new namespace using secrets
   # this setup can be done before the upgrade and will ensure the speediest creation of the necessary resources

   # auth for the control plane
   gcloud iam service-accounts keys create controller.json \
     --iam-account=$CONTROLLER_GSA@$PROJECT_ID.iam.gserviceaccount.com
   kubectl -n events-system create secret generic \
     google-cloud-key --from-file=key.json=controller.json
   # auth for the broker data plane
   gcloud iam service-accounts keys create broker.json \
     --iam-account=$BROKER_GSA@$PROJECT_ID.iam.gserviceaccount.com
   kubectl -n events-system create secret generic \
     google-broker-key --from-file=key.json=broker.json

   # remove the old webhook and controller to prevent them from interfering with the new ones
   kubectl delete deployment webhook -n cloud-run-events
   kubectl delete service webhook -n cloud-run-events
   kubectl delete deployment controller -n cloud-run-events
   kubectl delete service controller -n cloud-run-events
   ```

1. Upgrade to the new namespace by applying the changes from release `v0.20`.
   These instructions are identical to the
   [installation instructions](https://github.com/google/knative-gcp/blob/master/docs/install/install-knative-gcp.md#install-the-knative-gcp-constructs).

   ```shell script
   # apply changes based on the release
   export KGCP_VERSION=v0.20.0
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events-pre-install-jobs.yaml
   kubectl apply --selector events.cloud.google.com/crd-install=true --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   ```

### Upgrade for Authentication using Workload Identity

1. Run the following commands before installing the new `v0.20` release. We
   explain the purpose of each command in comments.

   ```shell script
   # create the new namespace by copying the labels and annotations from the old one
   kubectl get namespace cloud-run-events -o yaml | \
     sed 's/  name: cloud-run-events/  name: events-system/g' | \
     sed 's/\/cloud-run-events/\/events-system/g' | \
     kubectl apply -f -
   # copy the existing config maps to maintain existing configurations
   kubectl get configmaps -n cloud-run-events --field-selector metadata.name!=kube-root-ca.crt -o yaml | \
     sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
     sed 's/\/cloud-run-events\//\/events-system\//g' | \
     kubectl apply -f -

   # remove the old webhook and controller to prevent them from interfering with the new ones
   kubectl delete deployment webhook -n cloud-run-events
   kubectl delete service webhook -n cloud-run-events
   kubectl delete deployment controller -n cloud-run-events
   kubectl delete service controller -n cloud-run-events
   ```

1. Upgrade to the new namespace by applying the changes from release `v0.20`.
   These instructions are identical to the
   [installation instructions](https://github.com/google/knative-gcp/blob/master/docs/install/install-knative-gcp.md#install-the-knative-gcp-constructs).

   ```shell script
   # apply changes based on the release
   export KGCP_VERSION=v0.20.0
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events-pre-install-jobs.yaml
   kubectl apply --selector events.cloud.google.com/crd-install=true --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   ```

1. Update Workload Identity authentication to work in the new namespace. This
   can only be done after the upgrade above is executed, otherwise the
   Kubernetes Service Accounts in the new namespace will not be automatically
   created.

   ```shell script
   # auth for the control plane
   gcloud iam service-accounts add-iam-policy-binding \
     --role roles/iam.workloadIdentityUser \
     --member serviceAccount:$PROJECT_ID.svc.id.goog[$NEW_EVENTS_NS/controller] \
     $CONTROLLER_GSA@$PROJECT_ID.iam.gserviceaccount.com
   kubectl annotate serviceaccount -n $NEW_EVENTS_NS controller \
     iam.gke.io/gcp-service-account=$CONTROLLER_GSA@$PROJECT_ID.iam.gserviceaccount.com

   # auth for the broker data plane
   gcloud iam service-accounts add-iam-policy-binding \
     --role roles/iam.workloadIdentityUser \
     --member=serviceAccount:$PROJECT_ID.svc.id.goog[$NEW_EVENTS_NS/broker] \
     $BROKER_GSA@$PROJECT_ID.iam.gserviceaccount.com
   kubectl annotate serviceaccount -n $NEW_EVENTS_NS broker \
     iam.gke.io/gcp-service-account=$BROKER_GSA@$PROJECT_ID.iam.gserviceaccount.com

   # mark config-gcp-auth as initialized (important for the Cloud Console, which checks for this)
   kubectl annotate configmap  -n $NEW_EVENTS_NS  \
     config-gcp-auth --overwrite events.cloud.google.com/initialized=true

   ```

### Verify and Correct Existing Functionality

1. Confirm the new BrokerCell is working correctly.

   First ensure that the BrokerCell exists inside the `events-system` namespace.

   ```shell script
   # view all the resources in the events-system namespace
   kubectl get all -n events-system
   ```

   The expected output should include pods, deployments and replicas for the
   `controller`, `default-brokercell-fanout`, `default-brokercell-ingress`,
   `default-brokercell-retry`, and `webhook`; services for
   `controller`,`default-brokercell-ingress`, and `webhook`; and HPAs for
   `default-brokercell-fanout`, `default-brokercell-ingress`,
   `default-brokercell-retry`. For example:

   ```shell script
   NAME                                              READY   STATUS    RESTARTS   AGE
   pod/controller-5fbfcc975c-btjxr                   1/1     Running   0          11m
   pod/default-brokercell-fanout-54ff768b57-m9s6w    1/1     Running   0          11m
   pod/default-brokercell-ingress-74d78fd8bb-j9zgr   1/1     Running   0          11m
   pod/default-brokercell-retry-cf6479c65-fpm2k      1/1     Running   0          11m
   pod/webhook-7545c4fc5f-2tm4n                      1/1     Running   0          11m

   NAME                                 TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)           AGE
   service/controller                   ClusterIP   10.4.19.10    <none>        9090/TCP          16m
   service/default-brokercell-ingress   ClusterIP   10.4.25.144   <none>        80/TCP,9090/TCP   11m
   service/webhook                      ClusterIP   10.4.17.225   <none>        443/TCP           16m

   NAME                                         READY   UP-TO-DATE   AVAILABLE   AGE
   deployment.apps/controller                   1/1     1            1           16m
   deployment.apps/default-brokercell-fanout    1/1     1            1           11m
   deployment.apps/default-brokercell-ingress   1/1     1            1           11m
   deployment.apps/default-brokercell-retry     1/1     1            1           11m
   deployment.apps/webhook                      1/1     1            1           16m

   NAME                                                    DESIRED   CURRENT   READY   AGE
   replicaset.apps/controller-5fbfcc975c                   1         1         1       11m
   replicaset.apps/default-brokercell-fanout-54ff768b57    1         1         1       11m
   replicaset.apps/default-brokercell-ingress-74d78fd8bb   1         1         1       11m
   replicaset.apps/default-brokercell-retry-cf6479c65      1         1         1       11m
   replicaset.apps/webhook-7545c4fc5f                      1         1         1       11m

   NAME                                                                 REFERENCE                               TARGETS                   MINPODS   MAXPODS   REPLICAS   AGE
   horizontalpodautoscaler.autoscaling/default-brokercell-fanout-hpa    Deployment/default-brokercell-fanout    18575360/1000Mi, 0%/50%   1         10        1          11m
   horizontalpodautoscaler.autoscaling/default-brokercell-ingress-hpa   Deployment/default-brokercell-ingress   17760256/1500Mi, 0%/95%   1         10        1          11m
   horizontalpodautoscaler.autoscaling/default-brokercell-retry-hpa     Deployment/default-brokercell-retry     18268160/1000Mi, 0%/95%   1         10        1          11m
   ```

   Next, for each broker that you have previously deployed, make sure it is
   using the new BrokerCell ingress. Below we show an example where we examine
   the broker `test-broker` from the namespace `example`.

   ```shell script
   kubectl describe broker test-broker -n example
   ```

   The command output should contain the URL address
   `http://default-brokercell-ingress.events-system.svc.cluster.local/example/test-broker`
   as shown in the sample output below. Take note that if the URL is
   `http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/example/test-broker`
   you are still using the old broker installation.

   ```shell script
   Name:         test-broker
   Namespace:    example
   Labels:       <none>
   Annotations:  ...
   API Version:  eventing.knative.dev/v1
   Kind:         Broker
   Metadata:
      ...
   Spec:
      ...
   Status:
     Address:
       URL:  http://default-brokercell-ingress.events-system.svc.cluster.local/example/test-broker
     ...
   ```

1. Delete the old BrokerCell since it is no longer useful.

   ```shell script
   kubectl delete deployment default-brokercell-fanout -n cloud-run-events
   kubectl delete deployment default-brokercell-ingress -n cloud-run-events
   kubectl delete deployment default-brokercell-retry -n cloud-run-events
   kubectl delete service default-brokercell-ingress -n cloud-run-events
   kubectl delete horizontalpodautoscaler.autoscaling/default-brokercell-fanout-hpa -n cloud-run-events
   kubectl delete horizontalpodautoscaler.autoscaling/default-brokercell-ingress-hpa -n cloud-run-events
   kubectl delete horizontalpodautoscaler.autoscaling/default-brokercell-retry-hpa -n cloud-run-events
   # optional: remove post-install for v0.17.6
   kubectl delete job.batch/storage-version-migration-knative-gcp -n cloud-run-events
   ```

1. For each existing source delete the old source deployment. A new source
   deployment will be created automatically. The example below illustrates the
   process for a Cloud PubSub source `cloudpubsubsource-test` in the `example`
   namespace.

   ```shell script
   # view the deployment of an existing PubSub source
   kubectl describe deployment.apps/cre-src-cloudpubsubsource-test482579758ed75d60ed92997c7b3015bc -n example
   ```

   The output would look similar to the example below. Note the use of
   `cloud-run-events`.

   ```shell script
   Name:                   cre-src-cloudpubsubsource-test482579758ed75d60ed92997c7b3015bc
   Namespace:              example
   CreationTimestamp:      Thu, 07 Jan 2021 19:15:38 +0000
   Labels:                 internal.events.cloud.google.com/controller=cloud-run-events-pubsub-pullsubscription-controller
                           internal.events.cloud.google.com/pullsubscription=cloudpubsubsource-test
   Annotations:            deployment.kubernetes.io/revision: 1
                           metrics-resource-group: cloudpubsubsources.events.cloud.google.com
   Selector:               internal.events.cloud.google.com/controller=cloud-run-events-pubsub-pullsubscription-controller,internal.events.cloud.google.com/pullsubscription=cloudpubsubsource-test
   Replicas:               1 desired | 1 updated | 1 total | 1 available | 0 unavailable
   StrategyType:           RollingUpdate
   MinReadySeconds:        0
   RollingUpdateStrategy:  25% max unavailable, 25% max surge
   Pod Template:
     Labels:  internal.events.cloud.google.com/controller=cloud-run-events-pubsub-pullsubscription-controller
              internal.events.cloud.google.com/pullsubscription=cloudpubsubsource-test
   ...
   ```

   ```shell script
   # delete that deployment
   kubectl delete deployment.apps/cre-src-cloudpubsubsource-test482579758ed75d60ed92997c7b3015bc -n example
   ```

   A new deployment will be automatically generated using `events-system`.

   ```shell script
   # view the new deployment for the PubSub source
   kubectl describe deployment.apps/cre-src-cloudpubsubsource-test482579758ed75d60ed92997c7b3015bc -n example
   ```

   ```shell script
   Name:                   cre-src-cloudpubsubsource-test482579758ed75d60ed92997c7b3015bc
   Namespace:              example
   CreationTimestamp:      Thu, 07 Jan 2021 19:23:36 +0000
   Labels:                 internal.events.cloud.google.com/controller=events-system-pubsub-pullsubscription-controller
                           internal.events.cloud.google.com/pullsubscription=cloudpubsubsource-test
   Annotations:            deployment.kubernetes.io/revision: 1
                           metrics-resource-group: cloudpubsubsources.events.cloud.google.com
   Selector:               internal.events.cloud.google.com/controller=events-system-pubsub-pullsubscription-controller,internal.events.cloud.google.com/pullsubscription=cloudpubsubsource-test
   Replicas:               1 desired | 1 updated | 1 total | 1 available | 0 unavailable
   StrategyType:           RollingUpdate
   MinReadySeconds:        0
   RollingUpdateStrategy:  25% max unavailable, 25% max surge
   Pod Template:
     Labels:  internal.events.cloud.google.com/controller=events-system-pubsub-pullsubscription-controller
              internal.events.cloud.google.com/pullsubscription=cloudpubsubsource-test
   ...
   ```

1. Delete the old namespace after verifying that all resources have been
   migrated. Pay special attention to any custom resources that are not part of
   the events infrastructure since these were not covered by the steps above.

   You can check that all resources have been successfully deleted, by running:

   ```shell script
   # check what is left inside the old namespace
   kubectl get all -n cloud-run-events
   ```

   The expected output should be:

   ```
   No resources found in cloud-run-events namespace.
   ```

   If there are still custom resources inside the namespace, you should migrate
   them as well before deleting the now-redundant `cloud-run-events` namespace.

   ```shell script
   # delete the old namespace
   kubectl delete namespace cloud-run-events
   ```

1. Validate and resolve any hard-coded uses of the namespace in the user code to
   ensure no unintended impact. Search for the `cloud-run-events` keywords in
   any scripts you may have created and replace them with `events-system`. Note
   that any hardcoded uses of the old BrokerCell ingress
   `http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/...`
   will fail when attempting to send events.
