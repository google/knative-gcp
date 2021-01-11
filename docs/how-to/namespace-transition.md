# Transition a cluster to the new `events-system` namespace

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

1. Run the following commands before installing the new `v0.20` release. We
   explain the purpose of each command in comments.

   ```shell script
   # create the new namespace by copying the labels and annotations from the old one
   kubectl get namespace cloud-run-events -o yaml | \
     sed 's/  name: cloud-run-events/  name: events-system/g' | \
     kubectl create -f -
   # setup identical authentication by copying the existing secrets from the old namespace
   kubectl get secrets -n cloud-run-events -o yaml | \
     sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
     kubectl create -f -
   # copy the existing config maps to maintain existing configurations
   kubectl get configmaps -n cloud-run-events --field-selector metadata.name!=kube-root-ca.crt -o yaml | \
     sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
     kubectl create -f -

   # remove the old webhook and controller to prevent them from interfering with the new ones
   kubectl delete deployment webhook -n cloud-run-events
   kubectl delete service webhook -n cloud-run-events
   kubectl delete deployment controller -n cloud-run-events
   kubectl delete service controller -n cloud-run-events
   ```

   For your convenience, we also packaged these instructions as a shell script
   and as a yaml file that you can execute as shown below.

   ```shell script
   # execute the setup commands using the provided script
   bash namespace-transition.sh
   ```

   Or, if you prefer executing these commands as a job, apply the following yaml
   file.

   ```shell script
   # execute the setup commands as a job
   kubectl apply -f namespace-transition.yaml
   ```

   Once you confirm that the job has completed successfully, you can delete it.

   ```shell script
   # confirm that the job has completed successfully
   kubectl get all -n cloud-run-events
   ```

   ```shell script
   NAME                                              READY   STATUS        RESTARTS   AGE
   pod/namespace-transition-xwrxk                    0/1     Completed     0          3m3s

   NAME                                              COMPLETIONS   DURATION   AGE
   job.batch/namespace-transition                    1/1           37s        3m3s
   ```

   ```shell script
   # delete the job
   kubectl delete -f namespace-transition.yaml
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
