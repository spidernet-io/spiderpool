# Quick Start

*Let's start some Pods with Spiderpool in approximately 5 minutes.*

## Install Spiderpool

Set up the Helm repository.

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
```

Set up the environment variables of the default IPv4 IPPool for the cluster.

```bash
export IPV4_SUBNET_YOU_EXPECT="172.18.40.0/24"
export IPV4_IPRANGES_YOU_EXPECT="172.18.40.40-172.18.40.200"
```

> The default IPPool usually serves some components that need IP addresses when the Kubernetes cluster is initialized, such as CoreDNS.

Deploy Spiderpool with the following command.

```bash
helm install spiderpool spiderpool/spiderpool --namespace kube-system \
  --set spiderpoolController.tls.method=auto \
  --set feature.enableIPv4=true \
  --set feature.enableIPv6=false \
  --set clusterDefaultPool.installIPv4IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${IPV4_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv4IPRanges={${IPV4_IPRANGES_YOU_EXPECT}}
```

> During the deployment, the necessary TLS certificates and custom resources will be automatically created. See more details about [installation](./install.md).
>
> IPv6 is disabled in this case.
>
> If you want Spiderpool to work under dual stacks, set `feature.enableIPv6=true`.
>
> If you need to create a default IPv6 IPPool, set `clusterDefaultPool.installIPv6IPPool=true` and fill in the desired default IPv6 IPPool information in the `clusterDefaultPool.ipv6Subnet` and `clusterDefaultPool.ipv6IPRanges` parameters at the same time.

See more details about [Spiderpool helm charts parameters](https://github.com/spidernet-io/spiderpool/blob/main/charts/spiderpool/README.md#parameters).

## Update CNI network configuration

After the installation, you should update the
[CNI network configuration](https://www.cni.dev/docs/spec/#section-1-network-configuration-format)
under the default path `/etc/cni/net.d` on the Node where you want to use Spiderpool,
so that the Main CNI can use Spiderpool IPAM CNI to allocate IP addresses.

Replace the `ipam` object with the following to update it:

```json
"ipam":{
    "type":"spiderpool"
}
```

The following is an example configuration of [macvlan CNI](https://www.cni.dev/plugins/current/main/macvlan/):

```json
{
    "cniVersion":"0.4.0",
    "type":"macvlan",
    "mode":"bridge",
    "master":"eth0",
    "name":"macvlan",
    "ipam":{
        "type":"spiderpool"
    }
}
```

## Create an IPPool

Next, let's try to create an custom IPPool:

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ipv4-ippool.yaml
```

The YAML file looks like:

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: custom-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.50
```

You can replace `spec.subnet` and `spec.ips` as needed. See more details about [SpiderIPPool CRD](https://github.com/spidernet-io/spiderpool/blob/main/docs/concepts/spiderippool.md).

## Confirm the IPPool is working

To confirm the custom IPPool is working as expected, let's create a Deployment with 3 replicas:

```bash
kubectl create deployment my-dep --image=busybox --replicas=3 -- sleep infinity
```

Pods controlled by this Deployment will be assigned with IP addresses from the cluster's default IPPool and run as expected.

Check IPs of these pods with the following command:

```bash
kubectl get po -l app=my-dep -o wide
NAME                      READY   STATUS    RESTARTS   AGE     IP              NODE            NOMINATED NODE   READINESS GATES
my-dep-864946ffd8-h5z27   1/1     Running   0          3m10s   172.18.40.42    spider-worker   <none>           <none>
my-dep-864946ffd8-kdl86   1/1     Running   0          3m10s   172.18.40.200   spider-worker   <none>           <none>
my-dep-864946ffd8-vhnsj   1/1     Running   0          3m10s   172.18.40.38    spider-worker   <none>           <none>
```

## Allocate IP addresses from the custom IPPool

In addition to the cluster's default IPPool, Spiderpool also supports allocating IP addresses from custom IPPools.

You can use the following command and YAML file to specify the custom IPPool just created in the section [Create an IPPool](#create-an-ippool) to allocate IP addresses through Pod annotation `ipam.spidernet.io/ippool`. See more details about [pool selection rules](TODO).

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/basic/custom-ippool-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-ippool-deploy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: custom-ippool-deploy
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["custom-ipv4-ippool"]
          }
      labels:
        app: custom-ippool-deploy
    spec:
      containers:
      - name: custom-ippool-deploy
        image: busybox
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

As expected, Pods of Deployment `custom-ippool-deploy` will be assigned with IP addresses from IPPool `custom-ipv4-ippool`.
