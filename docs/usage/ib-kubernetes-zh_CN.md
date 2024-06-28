
# UFM 和 ib-kubernetes 

**简体中文** | [**English**](./ib-kubernetes.md)

## 介绍

[UFM](https://docs.nvidia.com/networking/display/ufmenterpriseqsgv6160/ufm+installation+steps) 是 Nvidia 用于管理 Infiniband 网络的软件，它可管理 Infiniband 的交换机以及主机。它具有以下的能力:

* 发现并管理 Infiniband 网络的主机、交换机、线缆、网关等网络设备，进行设备发现、拓扑展示、软件升级、日志采集等基础管理。
* 实施包括 pkey 在内的网络配置
* 网络设备状态监控，网络实时流量遥测，网络自检，实施告警
* opensm 管理网络设备

> 注： UFM 软件以 Server 形式运行在具有 Infiniband 网卡的主机上，需要购买 License 才能使用。

[ib-kubernetes](https://github.com/Mellanox/ib-kubernetes) 是 Mellanox 开源的一款 Kubernetes 的 Infiniband 插件，它与 [ib-sriov-cni](https://github.com/k8snetworkplumbingwg/ib-sriov-cni) 和 [Multus-cni](https://github.com/k8snetworkplumbingwg/multus-cni) 协同工作： 完成 Pkey 和 GUID 的设置，并通告给 UFM 类的插件， 再由 UFM 完成 Infiniband 网络下的子网管理。

下面将介绍 UFM 和 ib-kubernetes 如何协同工作以及一些使用实例。

## 工作流程

![ib-kubernetes-ufm](../images/ib-kubernetes.png)

从上图可以看出：ib-kubernetes 会在 Pod 创建时，读取它的 multus 配置或 annotations，从而读取 pkey 或 guid(如有配置，没有将自动生成)，通过调用 UFM 插件的 API，将 pkey 和 guid 信息传递给 UFM 插件。然后 UFM 将根据二者信息完成对该 Pod 的子网管理功能。

## 如何使用

您需要提前在环境中安装好 UFM 插件，下面我们安装 ib-kubernetes:

1. 准备好插件配置，以下配置用于帮助登录 UFM 管理平台。

        apiVersion: v1
        kind: Secret
        metadata:
          name: ib-kubernetes-ufm-secret
          namespace: kube-system
        stringData:
          UFM_USERNAME: "admin"  # UFM 用户名
          UFM_PASSWORD: "123456" # UFM 密码
          UFM_ADDRESS: ""        # UFM 管理地址 
          UFM_HTTP_SCHEMA: ""    # http/https. Default: https
          UFM_PORT: ""           # UFM REST API port. Defaults: 443(https), 80(http)
        string:
          UFM_CERTIFICATE: ""    # UFM Certificate in base64 format. (if not provided client will not

2. 登录 UFM 需要以证书方式，需要先在 UFM 主机生成证书文件：

        $ openssl req -x509 -newkey rsa:4096 -keyout ufm.key -out ufm.crt -days 365 -subj '/CN=<UFM hostname>'

        将证书文件复制到 UFM 证书位置：

        $ cp ufm.key /etc/pki/tls/private/ufmlocalhost.key
        $ cp ufm.crt /etc/pki/tls/certs/ufmlocalhost.crt

        重启 UFM：

        $ docker restart ufm

        如果以裸金属部署：

        $ systemctl restart ufmd

3. 创建 UFM 的证书密钥文件：

        $ kubectl create secret generic ib-kubernetes-ufm-secret --namespace="kube-system" --from-literal=UFM_USER="admin" --from-literal=UFM_PASSWORD="12345" --from-literal=UFM_ADDRESS="127.0.0.1" --from-file=UFM_CERTIFICATE=ufmlocalhost.crt --dry-run -o yaml > ib-kubernetes-ufm-secret.yaml
        $ kubectl create -f ./ib-kubernetes-ufm-secret.yaml 

4. 安装 ib-kubernetes:

        $ git clone https://github.com/Mellanox/ib-kubernetes.git && cd ib-kubernetes
        $ $ kubectl create -f deployment/ib-kubernetes-configmap.yaml
        $ kubectl create -f deployment/ib-kubernetes-ufm-secret.yaml
        $ kubectl create -f deployment/ib-kubernetes.yaml 

## 安装 Spiderpool

参考 [rdma-ib](./rdma-ib-zh_CN.md) 安装使用 Spiderpool, 我们只需要注意创建 SpiderMultusConfig 的时候指定 pkey 即可:

        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderMultusConfig
        metadata:
          name: ib-sriov
          namespace: kube-system
        spec:
          cniType: ib-sriov
          ibsriov:
            resourceName: spidernet.io/mellanoxibsriov
            pkey: 1000
            ippools:
              ipv4: ["v4-91"]

## 结论

ib-kubernetes 可与 UFM 软件集成，帮助 UFM 完成 Kubernetes 下 Infiniband 网络的子网管理。
