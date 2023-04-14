# Route support 

## Description

Spiderpool supports the configuration of routing information.

## Get Started

### Set up Spiderpool

follow the guide [installation](./get-started-macvlan.md) to install Spiderpool.

### Create Subnet

Create a SpiderSubnet and set up a subnet routes, a Pod and get the IP address from the AutoIPPool of the subnet, then see the routes configured in the subnet that exist in the AutoIPPool and Pod.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/subnet-route.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderSubnet
metadata:
  name: ipv4-subnet-route
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.41-172.18.41.60
  routes:
    - dst: 172.18.42.0/24
      gw: 172.18.41.1
```

### Create Deployment By SpiderSubnet

Create a Deployment whose Pods sets the Pod annotation `ipam.spidernet.io/subnet` to explicitly specify the subnet.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/subnet-route-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: subnet-test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: subnet-test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/subnet: |-
          {
            "ipv4": ["ipv4-subnet-route"]
          }
      labels:
        app: subnet-test-app
    spec:
      containers:
        - name: route-test
          image: busybox
          imagePullPolicy: IfNotPresent
          command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

Spiderpool has created fixed IP pools for applications, ensuring that the applications' IPs are automatically fixed within the defined ranges.

```bash
~# kubectl get spiderpool
NAME                                        VERSION   SUBNET           ALLOCATED-IP-COUNT   TOTAL-IP-COUNT   DEFAULT   DISABLE
auto-subnet-test-app-v4-eth0-d69f2fb7bccf   4         172.18.41.0/24   1                    1                false     false

~# kubectl get spiderpool auto-subnet-test-app-v4-eth0-d69f2fb7bccf -oyaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
...
  ips:
  - 172.18.41.41
  podAffinity:
    matchLabels:
      app: subnet-test-app
  routes:
  - dst: 172.18.42.0/24
    gw: 172.18.41.1
  subnet: 172.18.41.0/24
...
```

The Pods are running.

```bash
~# kubectl get pod -l app=subnet-test-app -owide
NAME                               READY   STATUS    RESTARTS   AGE     IP             NODE            NOMINATED NODE   READINESS GATES
subnet-test-app-59df44fc57-clp8t   1/1     Running   0          3m48s   172.18.41.41   spider-worker   <none>           <none>
```

After the created Pod has obtained an IP from the automatic IPPool, the route set in the subnet, which is inherited by the automatic pool and takes effect in the Pod, you can view it via IP r as follows:

```bash
~# kubectl exec -it route-test-app-bdc84f8f5-2bxbr  -- ip r
172.18.41.0/24 dev eth0 scope link  src 172.18.41.41 
172.18.42.0/24 via 172.18.41.1 dev eth0 
```

### Create IPPool

Create a SpiderIPPool and set up the routes for the IPPool, create the Pod and assign IP addresses from the IPPool, you can see the routing information in the ippool pool taking effect within the Pod.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/ippool-route.yaml
```

```yaml
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderIPPool
metadata:
  name: ipv4-ippool-route
spec:
  subnet: 172.18.41.0/24
  ips:
    - 172.18.41.51-172.18.41.60
  routes:
    - dst: 172.18.42.0/24
      gw: 172.18.41.1
```

### Create Deployment By IPPool

Create a Deployment whose Pods sets the Pod annotation `ipam.spidernet.io/ippool` to  explicitly specify the IPPool.

```bash
kubectl apply -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/ippool-route-deploy.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ippool-test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ippool-test-app
  template:
    metadata:
      annotations:
        ipam.spidernet.io/ippool: |-
          {
            "ipv4": ["ipv4-ippool-route"]
          }
      labels:
        app: ippool-test-app
    spec:
      containers:
        - name: route-test
          image: busybox
          imagePullPolicy: IfNotPresent
          command: ["/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"]
```

The Pods are running.

```bash
~# kubectl get pod -l app=ippool-test-app -owide
NAME                               READY   STATUS    RESTARTS   AGE   IP             NODE            NOMINATED NODE   READINESS GATES
ippool-test-app-66fd47d895-pthx5   1/1     Running   0          45s   172.18.41.53   spider-worker   <none>           <none>
```

After the created Pod has obtained an IP from IPPool, the route set in IPPool is already in effect in the Pod and you can view it via IP r as follows:

```bash
~# kubectl exec -it ippool-test-app-66fd47d895-pthx5  -- ip r
172.18.41.0/24 dev eth0 scope link  src 172.18.41.53 
172.18.42.0/24 via 172.18.41.1 dev eth0 
```

### Clean up

Clean the relevant resources so that you can run this tutorial again

   ```bash
   kubectl delete \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/subnet-route.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/subnet-route-deploy.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/ippool-route.yaml \
   -f https://raw.githubusercontent.com/spidernet-io/spiderpool/main/docs/example/route/ippool-route-deploy.yaml \
   --ignore-not-found=true
   ```
