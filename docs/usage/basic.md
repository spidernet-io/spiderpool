# Quick start

*Let's start some Pods with Spiderpool in approximately 5 minutes.*

## Install Spiderpool

First, add the Spiderpool `helm` repository.

```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
```

Set the IP addresses included in the cluster default IPv4 IPPool. The cluster default IPPool usually serves some components that need IP addresses when the Kubernetes cluster is initialized, such as CoreDNS.

```bash
IPV4_SUBNET_YOU_EXPECT="172.18.40.0/24"
IPV4_IPRANGES_YOU_EXPECT="172.18.40.40-172.20.40.200"
```

Install Spiderpool by creating the necessary TLS certificates and custom resources.

```bash
helm install spiderpool spiderpool/spiderpool --wait --namespace kube-system \
  --set spiderpoolController.tls.method=auto \
  --set feature.enableIPv4=true --set feature.enableIPv6=false \
  --set clusterDefaultPool.installIPv4IPPool=true  \
  --set clusterDefaultPool.ipv4Subnet=${IPV4_SUBNET_YOU_EXPECT} \
  --set clusterDefaultPool.ipv4IPRanges={${IPV4_IPRANGES_YOU_EXPECT}}
```

Here we use `--set feature.enableIPv6=false` to disable IPv6, more details of [Spiderpool charts values](https://github.com/spidernet-io/spiderpool/blob/main/charts/spiderpool/README.md#parameters).

## Update CNI network configuration

Then, edit the [CNI network configuration](https://www.cni.dev/docs/spec/#section-1-network-configuration-format) with the default path `/etc/cni/net.d` on the related Nodes so that the Main CNI can use Spiderpool IPAM CNI to allocate IP addresses. Replace the `ipam` object with the following:

```json
"ipam":{
    "type":"spiderpool"
}
```

Take a example of Macvlan + Spiderpool.

```json
{
    "cniVersion":"1.0.0",
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

Next, let's try to create an custom IPPool.

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/blob/main/docs/example/basic/custom-ipv4-ippool.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v1
kind: SpiderIPPool
metadata:
  name: custom-ipv4-ippool
spec:
  ipVersion: 4
  subnet: 172.18.41.0/24
  ips:
  - 172.18.41.40-172.18.41.50
```

We can replace `spec.subnet` and `spec.ips` as needed, more details of [SpiderIPPool CRD](https://github.com/spidernet-io/spiderpool/blob/main/docs/concepts/spiderippool.md).

## Run

Finally, create a Deployment with 3 replicas.

```bash
kubectl create deployment my-dep --image=busybox --replicas=3 -- sleep infinity
```

We will find that the Pods controlled by it allocate IP addresses from the cluster default IPPool and run successfully.

```bash
kubectl get po -l app=my-dep -o wide
NAME                      READY   STATUS    RESTARTS   AGE     IP              NODE            NOMINATED NODE   READINESS GATES
my-dep-864946ffd8-h5z27   1/1     Running   0          3m10s   172.18.40.42    spider-worker   <none>           <none>
my-dep-864946ffd8-kdl86   1/1     Running   0          3m10s   172.18.40.200   spider-worker   <none>           <none>
my-dep-864946ffd8-vhnsj   1/1     Running   0          3m10s   172.18.40.38    spider-worker   <none>           <none>
```

Of course, we can also specify the custom IPPool just created above to allocate IP addresses through Pod annotation `ipam.spidernet.io/ippool`, more details of [pool selection rules](TODO).

```bash
kubectl apply -f https://github.com/spidernet-io/spiderpool/blob/main/docs/example/basic/custom-ippool.deploy.yaml
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
            "interface": "eth0",
            "ipv4pools": ["custom-ipv4-ippool"]
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

As expected, Deployment `custom-ippool-deploy` 's Pods will allocate IP addresses from IPPool `custom-ipv4-ippool`.
