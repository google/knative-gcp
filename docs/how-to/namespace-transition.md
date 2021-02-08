# Upgrade cluster from eventing release v0.19 to v0.20

**This documentation is for clusters that already have release `v0.19` of
eventing running in the `cloud-run-events` namespace. If you are installing
eventing for the first time on your cluster, you do not need to follow this
steps and can go directly to
[Installing Knative-GCP](https://github.com/google/knative-gcp/blob/main/docs/install/install-knative-gcp.md).**

The upcoming `v0.20` release of `knative-gcp` will contain changes that are not
compatible with old releases. Namely, the namespace will change from
`cloud-run-events` to `events-system`. Using the new release without running the
migration steps described below will result in downtime and errors produced by
both the previous and current installation.

To avoid these issue we recommend taking the proactive approach described in
[Recommended Migration](#recommended-migration) to ensure continued good
functionality of your installed events system.

**Note that the [Recommended Migration](#recommended-migration) instructions
below cover installation based on the release from the `knative-gcp` repository.
[Recommended Migration for CloudRun](#recommended-migration-for-cloudrun)
contains instructions for the upgrade when using the
[Events for Cloud Run for Anthos](https://cloud.google.com/kuberun/docs/events/anthos/quickstart)
offering.**

## Recommended Migration

Migrating your events resources to the new namespace involves the following
steps:

1. Setting up authentication for the new namespace.
1. Upgrading to the new version of events.
1. Verifying the correct functionality of already deployed resources.

### Setup Authentication

We assume here that the desire is to keep the existing authentication mechanism
and provide the steps to correct the permissions to work in the new namespace.
These steps will transfer your cluster's authentication configuration to the new
namespace.

You will first need to create the new namespace. We recommend doing it by
copying the current one to keep any annotations or labels you have added.

```shell script
# create the new namespace by copying the labels and annotations from the old one
kubectl get namespace cloud-run-events -o yaml | \
  sed 's/  name: cloud-run-events/  name: events-system/g' | \
  sed 's!/cloud-run-events!/events-system!g' | \
  kubectl apply -f -
```

The expected output is:

```shell script
namespace/events-system created
```

You can verify that this command was completed correctly by listing all the
current namespaces in your cluster. You should be able to see `events-system`
among them.

```shell script
kubectl get namespaces
```

For example:

```shell script
NAME                       STATUS   AGE
cloud-run-events           Active   2m53s
cloud-run-events-example   Active   107s
cloud-run-system           Active   3m16s
default                    Active   4m57s
events-system              Active   12s
knative-eventing           Active   2m53s
knative-serving            Active   3m34s
kube-node-lease            Active   4m59s
kube-public                Active   4m59s
kube-system                Active   4m59s
```

Next, we recommend that you copy the ConfigMaps from the old namespace into the
new one. This will allow you to keep the configurations you have already set up.
Pay special attention to copying `config-gcp-auth` since this contains
information relating to authentication and may lead to disruptions in service if
not present in the new namespace.

```shell script
# copy the existing config maps to maintain existing configurations
kubectl get configmaps -n cloud-run-events --field-selector metadata.name!=kube-root-ca.crt -o yaml | \
  sed 's/  namespace: cloud-run-events/  namespace: events-system/g' | \
  sed 's!/cloud-run-events/!/events-system/!g' | \
  kubectl apply -f -
```

The expected output is:

```shell script
configmap/config-gcp-auth created
configmap/config-leader-election created
configmap/config-logging created
configmap/config-observability created
configmap/config-tracing created
configmap/default-brokercell-broker-targets created
```

Follow the upgrade instructions for either
[Kubernetes Secrets](#using-kubernetes-secrets) or
[Workload Identity](#using-workload-identity) depending on your cluster's
current authentication mechanism. After the upgrade is completed, verify that
everything is working correctly by following the steps in
[Verify and Correct Existing Functionality](#verify-and-correct-existing-functionality).

#### Using Kubernetes Secrets

1. Setup authentication for the new namespace you created above by copying
   existing secrets. By doing this step before the upgrade you will ensure the
   speediest creation of the necessary resources.

   ```shell script
   # auth for the control plane
   kubectl get secret google-cloud-key --namespace=cloud-run-events -o yaml | \
     grep -v '^\s*namespace:\s' | kubectl apply --namespace=events-system -f -
   # auth for the broker data plane
   kubectl get secret google-broker-key --namespace=cloud-run-events -o yaml | \
     grep -v '^\s*namespace:\s' | kubectl apply --namespace=events-system -f -
   # [optional] if the google-cloud-sources-key secret is present in cloud-run-events
   # auth for the sources data plane
   kubectl get secret google-cloud-sources-key --namespace=cloud-run-events -o yaml | \
     grep -v '^\s*namespace:\s' | kubectl apply --namespace=events-system -f -
   ```

   The expected output (assuming only the first two secrets are present) is:

   ```shell script
   secret/google-cloud-key created
   secret/google-broker-key created
   ```

   You can also verify that the secrets were copied by running:

   ```shell script
   kubectl get secrets -n events-system
   ```

   The output should contain:

   ```shell script
   NAME                  TYPE                                  DATA   AGE
   google-broker-key     Opaque                                1      98s
   google-cloud-key      Opaque                                1      98s
   ```

#### Using Workload Identity

1.  We need to first setup a number of environment variables based on the
    existing authentication settings.

    1.  If you used the same names for the Google Service Accounts (GSAs) as
        throughout the documentation in
        [Installing Knative-GCP ](https://github.com/google/knative-gcp/blob/release-0.19/docs/install/install-knative-gcp.md)
        and
        [Installing a Service Account for the Data Plane](https://github.com/google/knative-gcp/blob/release-0.19/docs/install/dataplane-service-account.md)
        you can directly define the environment variables as listed below.

        ```shell script
        # environment variable with the project ID the cluster is in
        export PROJECT_ID=<gcp-project-identifier>
        # GSA for the control plane
        export CONTROLLER_GSA=cloud-run-events
        # GSA for the broker data plane
        export BROKER_GSA=cre-dataplane
        ```

    2.  Otherwise, the following `gcloud` command can be used to view the
        existing Google Service Accounts (GSAs) and their roles in your GCP
        project.

        ```shell script
        # environment variable with the project ID the cluster is in
        export PROJECT_ID=<gcp-project-identifier>
        # view GSAs and roles
        gcloud projects get-iam-policy $PROJECT_ID
        ```

        Using this information, you can find the service accounts set up for the
        control plane and broker data plane. The control plane GSA should have
        the roles: `pubsub.admin`, `storage.admin`, `cloudscheduler.admin`,
        `logging.configWriter` and `logging.privateLogViewer`. The broker data
        plane GSA should have the roles: `pubsub.editor`,
        `monitoring.metricWriter` and `cloudtrace.agent`.

        Once you found the names for these service accounts, set the following
        environment variables. In this example the names of the GSAs are
        `events-controller-gsa` for the control plane and `events-broker-gsa`
        for the broker data plane.

        ```shell script
        # GSA for the control plane
        export CONTROLLER_GSA=events-controller-gsa
        # GSA for the broker data plane
        export BROKER_GSA=events-broker-gsa
        ```

1.  Next, setup authentication for the new namespace using Workload Identity. By
    doing this step before the upgrade you will ensure the speediest creation of
    the necessary resources.

    ```shell script
    # auth for the control plane
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member serviceAccount:$PROJECT_ID.svc.id.goog[events-system/controller] \
      $CONTROLLER_GSA@$PROJECT_ID.iam.gserviceaccount.com
    kubectl create serviceaccount --namespace events-system controller
    kubectl annotate serviceaccount -n events-system controller \
      iam.gke.io/gcp-service-account=$CONTROLLER_GSA@$PROJECT_ID.iam.gserviceaccount.com

    # auth for the broker data plane
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member=serviceAccount:$PROJECT_ID.svc.id.goog[events-system/broker] \
      $BROKER_GSA@$PROJECT_ID.iam.gserviceaccount.com
    kubectl create serviceaccount --namespace events-system broker
    kubectl annotate serviceaccount -n events-system broker \
      iam.gke.io/gcp-service-account=$BROKER_GSA@$PROJECT_ID.iam.gserviceaccount.com

    # mark config-gcp-auth as initialized (important for the Cloud Console, which checks for this)
    kubectl annotate configmap  -n events-system  \
      config-gcp-auth --overwrite events.cloud.google.com/initialized=true
    ```

    The output should look like:

    ```shell script
    Updated IAM policy for serviceAccount [wi-gsa-controller-3@<gcp-project-identifier>.iam.gserviceaccount.com].
    bindings:
    - members:
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[cloud-run-events/controller]
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[events-system/controller]
      role: roles/iam.workloadIdentityUser
    etag: BwW6EZCKxFQ=
    version: 1
    serviceaccount/controller created
    serviceaccount/controller annotated
    Updated IAM policy for serviceAccount [wi-gsa-broker-3@<gcp-project-identifier>.iam.gserviceaccount.com].
    bindings:
    - members:
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[cloud-run-events/broker]
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[events-system/broker]
      role: roles/iam.workloadIdentityUser
    etag: BwW6EZC29Gw=
    version: 1
    serviceaccount/broker created
    serviceaccount/broker annotated
    configmap/config-gcp-auth annotated
    ```

    You can also check that the IAM policy was updated correctly for each
    account as follows:

    ```shell script
    gcloud iam service-accounts get-iam-policy $CONTROLLER_GSA@$PROJECT_ID.iam.gserviceaccount.com
    # the output should look like
    bindings:
    - members:
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[cloud-run-events/controller]
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[events-system/controller]
      role: roles/iam.workloadIdentityUser
    etag: BwW6EU5vLXA=
    version: 1
    ```

    ```shell script
    gcloud iam service-accounts get-iam-policy $BROKER_GSA@$PROJECT_ID.iam.gserviceaccount.com
    # the output should look like
    bindings:
    - members:
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[cloud-run-events/broker]
      - serviceAccount:<gcp-project-identifier>.svc.id.goog[events-system/broker]
      role: roles/iam.workloadIdentityUser
    etag: BwW6EU7UHUI=
    version: 1
    ```

    ```shell script
    kubectl describe serviceaccount/controller -n events-system
    # the output should look like
    Name:                controller
    Namespace:           events-system
    Labels:              <none>
    Annotations:         iam.gke.io/gcp-service-account: wi-gsa-controller-3@<gcp-project-identifier>.iam.gserviceaccount.com
    Image pull secrets:  <none>
    Mountable secrets:   controller-token-qwxgt
    Tokens:              controller-token-qwxgt
    Events:              <none>
    ```

    ```shell script
    kubectl describe serviceaccount/broker -n events-system
    # the output should look like
    Name:                broker
    Namespace:           events-system
    Labels:              <none>
    Annotations:         iam.gke.io/gcp-service-account: wi-gsa-broker-3@<gcp-project-identifier>.iam.gserviceaccount.com
    Image pull secrets:  <none>
    Mountable secrets:   broker-token-z6rlk
    Tokens:              broker-token-z6rlk
    Events:              <none>

    ```

### Upgrade to the Eventing v0.20 Release

1. Upgrade to the new namespace by applying the changes from release `v0.20`.
   These instructions are identical to the
   [installation instructions](https://github.com/google/knative-gcp/blob/main/docs/install/install-knative-gcp.md#install-the-knative-gcp-constructs).

   ```shell script
   # remove the old webhook and controller to prevent them from interfering with the new ones
   kubectl delete deployment webhook -n cloud-run-events
   kubectl delete service webhook -n cloud-run-events
   kubectl delete deployment controller -n cloud-run-events
   kubectl delete service controller -n cloud-run-events

   # apply changes based on the release
   export KGCP_VERSION=v0.20.0
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events-pre-install-jobs.yaml
   kubectl apply --selector events.cloud.google.com/crd-install=true --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   kubectl apply --filename https://github.com/google/knative-gcp/releases/download/${KGCP_VERSION}/cloud-run-events.yaml
   ```

### Verify and Correct Existing Functionality

1. **If you have brokers deployed.** Confirm the new BrokerCell is working
   correctly.

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

1. Delete the old BrokerCell since it is no longer used. This will not cause any
   service disruptions since the previous step already verified that the new
   BrokerCell is ready and active.

   ```shell script
   kubectl delete brokercell default -n cloud-run-events
   ```

1. **If you have sources deployed.** For each existing source delete the old
   source deployment. A new source deployment will be created automatically.

   You can view the deployed sources using the following commands:

   ```shell script
   # list all the sources in the cluster
   kubectl get sources --all-namespaces
   # list all the source deployments linked to the old cloud-run-events namespace
   kubectl get deploy --all-namespaces \
        --selector internal.events.cloud.google.com/controller=cloud-run-events-pubsub-pullsubscription-controller
   ```

   In the sample output below we have a single Cloud PubSub source with a
   deployment that uses the old namespace.

   ```shell script
   NAMESPACE   NAME                                                               READY   REASON   AGE
   default     cloudpubsubsource.events.cloud.google.com/cloudpubsubsource-test   True             61s
   NAMESPACE   NAME                                                             READY   UP-TO-DATE   AVAILABLE   AGE
   default     cre-src-cloudpubsubsource-test482579758ed75d60ed92997c7b3015bc   1/1     1            1           93s
   ```

   You can examine each source deployment that was identified by the command
   above to still be using the old `cloud-run-events` namespace. The example
   below illustrates the process for a Cloud PubSub source
   `cloudpubsubsource-test` in the `example` namespace.

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
   # delete all the source deployments referencing the old cloud-run-events namespace
   kubectl delete deploy --all-namespaces \
     --selector internal.events.cloud.google.com/controller=cloud-run-events-pubsub-pullsubscription-controller
   ```

   A new deployment will be automatically generated using `events-system` for
   each source.

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

   If there are remaining custom resources inside the namespace, you should
   migrate them as well before deleting the now-redundant `cloud-run-events`
   namespace.

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

## Recommended Migration for CloudRun

The "Events for Cloud Run for Anthos" installation of eventing will be upgraded
to the new namespace automatically.

If you are using authentication based on Kubernetes Secrets everything will be
migrated for you. However, we recommend that you run the
[authentication steps](#using-kubernetes-secrets) just to ensure the speediest
upgrade possible.

**If you are using authentication based on Workload Identity, it is critical
that you perform the [authentication steps](#using-workload-identity) for the
new namespace before the release becomes available.** This type of
authentication requires project admin permissions and cannot be performed on
your behalf by the upgrade.

In both cases we highly recommend that you apply any changes like labels and
annotations to both namespaces. Also, if you are editing ConfigMaps, please make
sure the changes are made in both namespaces before the upgrade. If this is not
done consistently you may need to apply these changes after the upgrade to get
back to your desired configurations.
