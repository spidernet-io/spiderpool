# Dynamic-Resource-Allocation

## Introduction

Dynamic-Resource-Allocation (DRA) is a new feature introduced by Kubernetes that puts resource scheduling in the hands of third-party developers. It provides an API more akin to a storage persistent volume, instead of the countable model (e.g., "nvidia.com/gpu: 2") that device-plugin used to request access to resources, with the main benefit being a more flexible and dynamic allocation of hardware resources, resulting in improved resource utilization. The main benefit is more flexible and dynamic allocation of hardware resources, which improves resource utilization and enhances resource scheduling, enabling Pods to schedule the best nodes. DRA is currently available as an alpha feature in Kubernetes 1.26 (December 2022 release), driven by Nvidia and Intel.
Spiderpool currently integrates with the DRA framework, which allows for the following, but not limited to:

* Automatically scheduling to the appropriate node based on the NIC and subnet information reported by each node, combined with the SpiderMultusConfig configuration used by the Pod, so as to prevent the Pod from not being able to start up after scheduling to the node.
* Unify the resource usage of multiple device-plugins: [sriov-network-device-plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin), [k8s-rdma-shared-dev-plugin](https://github.com/Mellanox/k8s-rdma-shared-dev-plugin) in the SpiderClaimParameter.
* Continuously updated, see for details. [RoadMap](../develop/roadmap.md)

## Explanation of nouns

* ResourceClaimTemplate: resourceclaim template for generating resourceclaim resources. One resourceClaimTemplate can generate multiple resourceclaims.
* ResourceClaim: ResourceClaim binds a specific set of node resources for use by the Pod.
* ResourceClass: A ResourceClass represents a resource (e.g., GPU), and a DRA plugin is responsible for driving the resource represented by a ResourceClass.

## Environment Preparation

* Prepare a Kubernetes cluster with a higher version than v1.29.0, and enable the dra feature-gate function of the cluster.
* Have Kubectl, [Helm] (<https://helm.sh/docs/intro/install/>) installed.

## Quick Start

1. Currently DRA is not turned on by default as an alpha feature of Kubernetes. So we need to turn it on manualways， as following steps.

    Add the following to the kube-apiserver startup parameters.

    ```shell
        --feature-gates=DynamicResourceAllocation=true
        --runtime-config=resource.k8s.io/v1alpha2=true
    ```

    Add the following to the kube-controller-manager startup parameters.

    ```shell
        --feature-gates=DynamicResourceAllocation=true
    ```

    Add the following to kube-scheduler's startup parameters:

    ```shell
        --feature-gates=DynamicResourceAllocation=true
    ```

2. DRA needs to rely on [CDI] (<https://github.com/cncf-tags/container-device-interface>), so it needs container runtime support. In this article, we take containerd as an example, and we need to enable cdi function manually.

    Modify the containerd configuration file to configure CDI.

    ```shell
    ~# vim /etc/containerd/config.toml
    ...
    [plugins. "io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
    ~# systemctl restart containerd
    ```

    > It is recommended that containerd be older than v1.7.0, as CDI is supported in later versions. The version supported by different runtimes is not the same, please check if it is supported first.

3. Install Spiderpool, taking care to enable CDI.

    ```shell
    helm repo add spiderpool https://spidernet-io.github.io/spiderpool
    helm repo update spiderpool
    helm install spiderpool spiderpool/spiderpool --namespace kube-system --set dra.enabled=true 
    
4. Verify the installation

    Check that the Spiderpool pod is running correctly, and check for the presence of the resourceclass resource:

    ```shell
    ~# kubectl get po -n kube-system | grep spiderpool
    spiderpool-agent-hqt2b 1/1 Running 0 20d
    spiderpool-agent-nm9vl 1/1 Running 0 20d
    spiderpool-controller-7d7f4f55d4-w2rv5 1/1 Running 0 20d
    spiderpool-init 0/1 Completed 0 21d
    ~# kubectl get resourceclass
    NAME                      DRIVERNAME                AGE
    netresources.spidernet.io netresources.spidernet.io 20d
    ```

    > netresources.spidernet.io is Spiderpool's resourceclass, and Spiderpool will take care of creating and allocating resourceclaims belonging to this resourceclass.

5. Create SpiderIPPool and SpiderMultusConfig instances.

    > Note: This step can be skipped if your cluster already has other CNIs installed or does not require an underlay CNI with Macvlan.

    ```shell
    MACVLAN_MASTER_INTERFACE="eth0"
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata: name: macvlan-config
      name: macvlan-conf
      namespace: kube-system
    metadata: name: macvlan-conf namespace: kube-system
      cniType: macvlan
      macvlan.
        master: ${MACVLAN_MASTER_INTERFACE}
        - ${MACVLAN_MASTER_INTERFACE}
    EOF
    ```

    > SpiderMultusConfig will automatically create the Multus network-attachment-definetion instance

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderIPPool
    metadata: name: ippool-test
      name: ippool-test
    spec.
      ips.
      - "172.18.30.131-172.18.30.140"
      subnet: 172.18.0.0/16
      gateway: 172.18.0.1
      multusName. 
      - kube-system/macvlan-conf
    EOF
    ``

6. Create resource files such as workloads and resourceClaim.

    ```shell
    ~# export NAME=demo
    ~# cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderClaimParameter
    metadata:
      name: ${NAME}
    ---
    apiVersion: resource.k8s.io/v1alpha2
    kind: ResourceClaimTemplate
    metadata:
      name: ${NAME}
    spec:
      spec:
        resourceClassName: netresources.spidernet.io
        parametersRef:
          apiGroup: spiderpool.spidernet.io
          kind: SpiderClaimParameter
          name: ${NAME}
    ---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: ${NAME}
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: ${NAME}
      template:
        metadata:
          annotations:
            v1.multus-cni.io/default-network: kube-system/macvlan-conf
        labels:
            app: ${NAME}
        spec:
          containers:
          - name: ctr
            image: nginx
            resources:
              claims:
              - name: ${NAME}
          resourceClaims:
          - name: ${NAME}
            source:
              resourceClaimTemplateName: ${NAME}
    EOF
    ```

    > Create a ResourceClaimTemplate, K8s will create its own unique Resourceclaim for each Pod based on this ResourceClaimTemplate. the declaration cycle of the Resourceclaim will be consistent with that of the Pod. The declaration cycle of the Resourceclaim is consistent with that of the Pod.
    >
    > The SpiderClaimParameter is used to extend the configuration parameters of the ResourceClaim, which will affect the scheduling of the ResourceClaim and the generation of its CDI file. In this example, setting rdmaAcc to true will affect whether or not the configured so file is mounted.
    >
    > A Pod's container affects the resources required by containerd by declaring the use of claims in Resources. The CDI file corresponding to the claim is translated into an OCI Spec configuration when the container is run, which determines the container's creation.
    >
    > If the Pod creation fails with "unresolvable CDI devices: xxxx", it is possible that the CDI version supported by the container at runtime is too low, which makes the container unable to parse the cdi file at runtime. Currently, the default CDI version of Spiderpool is the latest one. You can specify a lower version in the SpiderClaimParameter instance via annotation: "dra.spidernet.io/cdi-version", e.g.: dra.spidernet.io/cdi-version: 0.5.0

7. Validation

    After creating the Pod, view the generated resource files such as ResourceClaim.

    ```shell
    ~# kubectl get resourceclaim
    NAME                                                           RESOURCECLASSNAME           ALLOCATIONMODE         STATE                AGE
    demo-745fb4c498-72g7g-demo-7d458                               netresources.spidernet.io   WaitForFirstConsumer   allocated,reserved   20d
    ~# cat /var/run/cdi/k8s.netresources.spidernet.io-claim_1e15705a-62fe-4694-8535-93a5f0ccf996.yaml
    ---
    cdiVersion: 0.6.0
    containerEdits: {}
    devices:
    - containerEdits:
        env:
        - DRA_CLAIM_UID=1e15705a-62fe-4694-8535-93a5f0ccf996
      name: 1e15705a-62fe-4694-8535-93a5f0ccf996
    kind: k8s.netresources.spidernet.io/claim 
    ```

    This shows that the ResourceClaim has been created, and STATE shows allocated and reserverd, indicating that it has been used by the pod. And spiderpool has generated a CDI file for the ResourceClaim, which describes the files and environment variables to be mounted.

    Check that the pod is Running and verify that the the environment variable (DRA_CLAIM_UID) is declared.

    ```shell
    ~# kubectl get po
    NAME                        READY   STATUS    RESTARTS      AGE
    nginx-745fb4c498-72g7g      1/1     Running   0             20m
    nginx-745fb4c498-s92qr      1/1     Running   0             20m
    ~# kubectl exec -it nginx-745fb4c498-72g7g sh
    ~# printenv DRA_CLAIM_UID
    1e15705a-62fe-4694-8535-93a5f0ccf996
    ```

    You can see that the Pod's containers have correctly declared environment variables, It shows the dra is works.

## Welcome to try it out

DRA is currently available as an alpha feature of Spiderpool, and we'll be expanding it with more capabilities in the future, so feel free to try it out. Please let us know if you have any further questions or requests.
