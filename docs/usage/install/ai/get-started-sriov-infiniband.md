# AI Cluster With SR-IOV (InfiniBand)

**⚠️ Before proceeding, make sure your environment meets the [Requirements](./index.md#requirements), and finish the host preparation for InfiniBand RDMA mode in [Host preparation](./index.md#host-preparation).**

## Configure SR-IOV operator

The common end-to-end workflow for SR-IOV operator configuration is the same as RoCE. Please refer to [AI Cluster With SR-IOV(RoCE)](./get-started-sriov-roce.md) for the general workflow. In InfiniBand scenarios, note the following differences:

- When creating `SriovNetworkNodePolicy`, you must set `linkType: ib`.

Example using `eno3np2`:

```shell
$ LINK_TYPE=ib NIC_NAME=eno3np2 VF_NUM=12
$ cat <<EOF | kubectl apply -f -
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetworkNodePolicy
metadata:
  name: ib-${NIC_NAME}
  namespace: spiderpool
spec:
  nodeSelector:
    kubernetes.io/os: "linux"
  resourceName: eno3np2
  priority: 99
  numVfs: ${VF_NUM}
  nicSelector:
    pfNames:
      - ${NIC_NAME}
  linkType: ${LINK_TYPE}
  deviceType: netdevice
  isRdma: true
EOF
```

## Configure Spiderpool resources

Create CNI config and the corresponding IPPool (IB-SRIOV CNI):

```shell
$ cat <<EOF | kubectl apply -f -
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: gpu1-net11
spec:
  gateway: 172.16.11.254
  subnet: 172.16.11.0/16
  ips:
    - 172.16.11.1-172.16.11.200
---
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: gpu1-ib-sriov
  namespace: spiderpool
spec:
  cniType: ib-sriov
  ibsriov:
    resourceName: spidernet.io/eno3np2
    rdmaIsolation: true
    ippools:
      ipv4: ["gpu1-net11"]
EOF
```

> If the `ib-kubernetes` component is deployed in the cluster and integrated with a UFM management platform, configuring an IPPool for `SpiderMultusConfig` is optional.

## (Optional) Integrate with UFM in InfiniBand networks

For InfiniBand clusters with a [UFM management platform](https://www.nvidia.com/en-us/networking/infiniband/ufm/), you can deploy [ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) as a DaemonSet. It monitors Pods using SR-IOV NICs and reports VF PKey and GUID to UFM.

1. Create certificates on the UFM host:

    ```shell
    # replace to right address
    $ UFM_ADDRESS=172.16.10.10
    $ openssl req -x509 -newkey rsa:4096 -keyout ufm.key -out ufm.crt -days 365 -subj '/CN=${UFM_ADDRESS}'

    # Copy the certificate files to the UFM certificate directory:
    $ cp ufm.key /etc/pki/tls/private/ufmlocalhost.key
    $ cp ufm.crt /etc/pki/tls/certs/ufmlocalhost.crt

    # For containerized UFM deployment, restart the container service
    $ docker restart ufm

    # For host-based UFM deployment, restart the UFM service
    $ systemctl restart ufmd
    ```

2. Create the secret required by ib-kubernetes on Kubernetes. Copy `ufm.crt` from the UFM host to a Kubernetes node, then run:

    ```shell
    # replace to right user
    $ UFM_USERNAME=admin

    # replace to right password
    $ UFM_PASSWORD=12345

    # replace to right address
    $ UFM_ADDRESS="172.16.10.10"
    $ kubectl create secret generic ib-kubernetes-ufm-secret --namespace="kube-system" \
      --from-literal=UFM_USER="${UFM_USERNAME}" \
      --from-literal=UFM_PASSWORD="${UFM_PASSWORD}" \
      --from-literal=UFM_ADDRESS="${UFM_ADDRESS}" \
      --from-file=UFM_CERTIFICATE=ufm.crt
    ```

3. Install ib-kubernetes:

    ```shell
    git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
    $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
    kubectl create -f deployment/ib-kubernetes.yaml
    ```

4. When creating `SpiderMultusConfig` for InfiniBand, you can configure `pkey`. Pods created with this config will take effect, and will be synced to UFM by ib-kubernetes:

    ```shell
    $ cat <<EOF | kubectl apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderMultusConfig
    metadata:
      name: ib-sriov
      namespace: spiderpool
    spec:
      cniType: ib-sriov
      ibsriov:
        pkey: 1000
        ...
    EOF
    ```

    > Note: In an InfiniBand Kubernetes deployment, each node can be associated with up to 128 PKeys due to a kernel limitation.

## Create a test application

1. Create a DaemonSet on specified nodes to validate SR-IOV devices

    The following example uses annotation `v1.multus-cni.io/default-network` to specify the default Calico interface for control-plane communication; uses `k8s.v1.cni.cncf.io/networks` to attach 8 GPU-affinity VF interfaces for RDMA traffic; and requests 8 RDMA resources.

    > Tip: Webhook-based automatic injection is supported. See [Webhook-based Automatic RDMA Resource Injection](./get-started-sriov-roce.md#webhook-based-automatic-rdma-resource-injection).

    ```shell
    $ helm repo add spiderchart https://spidernet-io.github.io/charts
    $ helm repo update
    $ helm search repo rdma-tools
   
    # run daemonset on worker1 and worker2
    $ cat <<EOF > values.yaml
    # for china user , it could add these to use a domestic registry
    #image:
    #  registry: ghcr.m.daocloud.io

    # just run daemonset in nodes 'worker1' and 'worker2'
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/hostname
              operator: In
              values:
              - worker1
              - worker2

    # sriov interfaces
    extraAnnotations:
      k8s.v1.cni.cncf.io/networks: |-
        [{"name":"gpu1-sriov","namespace":"spiderpool"},
        {"name":"gpu2-sriov","namespace":"spiderpool"},
        {"name":"gpu3-sriov","namespace":"spiderpool"},
        {"name":"gpu4-sriov","namespace":"spiderpool"},
        {"name":"gpu5-sriov","namespace":"spiderpool"},
        {"name":"gpu6-sriov","namespace":"spiderpool"},
        {"name":"gpu7-sriov","namespace":"spiderpool"},
        {"name":"gpu8-sriov","namespace":"spiderpool"}]

    # sriov resource
    resources:
      limits:
        spidernet.io/gpu1sriov: 1
        spidernet.io/gpu2sriov: 1
        spidernet.io/gpu3sriov: 1
        spidernet.io/gpu4sriov: 1
        spidernet.io/gpu5sriov: 1
        spidernet.io/gpu6sriov: 1
        spidernet.io/gpu7sriov: 1
        spidernet.io/gpu8sriov: 1
        #nvidia.com/gpu: 1
    EOF

    $ helm install rdma-tools spiderchart/rdma-tools -f ./values.yaml
    
    ```

    During container network namespace creation, Spiderpool performs gateway connectivity checks on SR-IOV interfaces. If all Pods start successfully, the VFs are reachable and RDMA communication should work.

2. Check a Pod’s network namespace

    Enter any Pod to confirm it has 9 interfaces:

    ```shell
    $ kubectl exec -it rdma-tools-4v8t8  bash
    kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
    root@rdma-tools-4v8t8:/# ip a
      ...
    ```

    Confirm that 8 RDMA devices are available:

    ```shell
    root@rdma-tools-4v8t8:/# rdma link
        link mlx5_27/1 state ACTIVE physical_state LINK_UP netdev net2
        link mlx5_54/1 state ACTIVE physical_state LINK_UP netdev net1
        link mlx5_67/1 state ACTIVE physical_state LINK_UP netdev net4
        link mlx5_98/1 state ACTIVE physical_state LINK_UP netdev net3
        .....
    ```

3. Validate RDMA connectivity between cross-node Pods

    In one Pod, start the service:

    ```shell
    # see 8 RDMA devices assigned to the Pod
    $ rdma link

    # Start an RDMA service
    $ ib_read_lat
    ```

    In another Pod, access the service:

    ```shell
    # You should be able to see all RDMA network cards on the host
    $ rdma link
        
    # Successfully access the RDMA service of the other Pod
    $ ib_read_lat 172.91.0.115
    ```

    You can observe RDMA traffic statistics via `rdma statistic` in the container or refer to [RDMA metrics](../../rdma-metrics.md).

## Others

If you want to customize MTU for SR-IOV InfiniBand VFs, refer to:
[Customize VF MTU](./get-started-sriov-roce.md#customize-vf-mtu)
