# E2E Cases for IaaS Network Provider

| Case ID | Title | Priority | Smoke | Status | Other |
|---------|-------|----------|-------|--------|-------|
| E00019  | ENI slot device plugin should schedule Pods only up to the advertised node capacity. | p1 |       | done | merged from ENI device plugin suite |
| E00031  | Node allocatable should report the configured ENI slot total. | p1 |       | done | merged from ENI device plugin suite |
| E00043  | ENI slot capacity should be schedulable again after Pod deletion. | p1 |       | done | merged from ENI device plugin suite |
| E00044  | ENI slot allocatable should recover after spiderpool-agent restarts. | p2 |       | done | merged from ENI device plugin suite |
| I00001  | Create SpiderIPPool and VLAN SpiderMultusConfig, create three Pods on the same node with defaultMaxCount=2, verify two Pods run and one Pod is unschedulable, verify ENI slot resource injection, provider allocate and release calls, and compare provider ip-cache mac/vlan with SpiderEndpoint. | p1 |       | done | requires mock provider server |
| US2     | Network resource plugin should reconcile sub-ENI allocatable after node max-count annotation changes and schedule master-NIC Pods only onto matching nodes. | p1 |       | done | included in provider target via `networkresourceplugin` label |

## Mock Provider Server

The provider e2e target builds the mock IaaS provider image locally, loads it into kind, and the `iaasnetworkprovider` suite deploys the mock server before running cases.

The mock server image is built from:

```bash
make build_iaas_provider_mock_image \
    -e E2E_IAAS_PROVIDER_MOCK_IMAGE=spiderpool-iaas-provider-mock:latest
```

When using a local kind cluster, load the locally built image into the cluster:

```bash
kind load docker-image spiderpool-iaas-provider-mock:latest --name spider
```

The mock server implements:

```text
POST /v1/apis/network.iaas.io/ipam/allocate-ips
POST /v1/apis/network.iaas.io/ipam/release-ip
GET  /v1/apis/network.iaas.io/status/ips-cache/{ipAddress}
GET  /records
POST /reset
GET  /healthz
```

The `ips-cache` status API returns:

```json
{
  "entry": {
    "ipAddress": "10.0.0.10",
    "nodeName": "worker-01",
    "parentNicMac": "fa:16:3e:11:22:33",
    "subnet": "10.0.0.0/24",
    "mac": "fa:16:3e:aa:bb:01",
    "vlanID": 2101
  }
}
```

## Run Locally

Initialize the kind cluster for provider tests. This step configures the VLAN test network, keeps the dummy NICs used by master-NIC scheduling cases, and installs Spiderpool with IaaS provider integration enabled:

```bash
make e2e_init_iaasnetworkprovider \
    -e E2E_CLUSTER_NAME=spider \
    -e E2E_IAAS_PROVIDER_URL=http://provider-mock-server.iaas-provider-mock.svc:8080 \
    -e E2E_IAAS_PROVIDER_ENI_MAX_SLOTS_PER_NODE=2 \
    -e E2E_IAAS_PROVIDER_INJECT_POD_ENI_RESOURCES=true
```

Run the provider-related suites. This target builds and loads the local mock-server image automatically, then runs the `iaasnetworkprovider` and `networkresourceplugin` labeled tests in an IPv4-only cluster:

```bash
make e2e_test_iaasnetworkprovider \
    -e E2E_CLUSTER_NAME=spider \
    -e E2E_IAAS_PROVIDER_MOCK_IMAGE=spiderpool-iaas-provider-mock:latest \
    -e E2E_IAAS_PROVIDER_ENI_MAX_SLOTS_PER_NODE=2
```

Equivalent label filter used by the target:

```text
iaasnetworkprovider || networkresourceplugin
```
