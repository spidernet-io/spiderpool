# Quick Start: WorkloadEndpoint Query API

**Feature**: 002-workloadendpoint-query-api  
**Date**: 2026-04-27

## Overview

This API allows external systems to query Pod network allocation details (IP, VLAN, MAC) via Unix Socket. It's designed for scenarios like:
- CNI plugins retrieving allocation details after IPAM delegates to external cloud providers
- Network controllers querying Pod network configuration
- Monitoring systems collecting network topology information

## Prerequisites

- Spiderpool Agent running with Unix Socket enabled
- Access permission to `/var/run/spidernet/spiderpool.sock`
- Pod with IP allocation recorded in SpiderEndpoint CRD

## Querying the API

### Basic Query

```bash
curl --unix-socket /var/run/spidernet/spiderpool.sock \
  "http://localhost/v1/workloadendpoint?podNamespace=default&podName=my-pod"
```

### Response Examples

#### Standard Deployment (IP only)

```json
{
  "podNamespace": "default",
  "podName": "my-pod",
  "podUID": "abc-123-def-456",
  "node": "worker-1",
  "interfaces": [
    {
      "interface": "eth0",
      "ipv4": "10.244.1.100/24",
      "ipv6": "fd00:10:244::100/64",
      "ipv4Pool": "default-v4-pool",
      "ipv6Pool": "default-v6-pool",
      "ipv4Gateway": "10.244.1.1",
      "ipv6Gateway": "fd00:10:244::1"
    }
  ]
}
```

#### Public Cloud Integration (IP + MAC + VLAN)

```json
{
  "podNamespace": "default",
  "podName": "my-pod",
  "podUID": "abc-123-def-456",
  "node": "worker-1",
  "interfaces": [
    {
      "interface": "eth0",
      "ipv4": "10.244.1.100/24",
      "ipv6": "fd00:10:244::100/64",
      "mac": "aa:bb:cc:dd:ee:ff",
      "vlan": 100,
      "ipv4Pool": "cloud-vlan100-pool",
      "ipv6Pool": "cloud-vlan100-v6-pool",
      "ipv4Gateway": "10.244.1.1",
      "ipv6Gateway": "fd00:10:244::1"
    },
    {
      "interface": "net1",
      "ipv4": "192.168.1.50/24",
      "mac": "aa:bb:cc:dd:ee:00",
      "vlan": 200,
      "ipv4Pool": "sriov-vlan200-pool"
    }
  ]
}
```

### Error Responses

#### 404 - Pod Not Found

```json
{
  "error": "SpiderEndpoint not found for pod default/my-pod"
}
```

#### 400 - Missing Parameters

```json
{
  "error": "Missing required parameter: podNamespace"
}
```

## Go Client Example

```go
package main

import (
    "context"
    "fmt"
    "net"
    "net/http"
    
    openapi "github.com/spidernet-io/spiderpool/api/v1/agent/client"
)

func main() {
    // Create Unix Socket HTTP client
    httpClient := &http.Client{
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
                return net.Dial("unix", "/var/run/spidernet/spiderpool.sock")
            },
        },
    }
    
    // Create client
    client := openapi.New(openapi.DefaultHost, "", nil)
    client.SetTransport(httpClient.Transport)
    
    // Query workload endpoint
    params := daemonset.NewGetWorkloadendpointParams()
    params.WithPodNamespace("default")
    params.WithPodName("my-pod")
    
    resp, err := client.Daemonset.GetWorkloadendpoint(params)
    if err != nil {
        panic(err)
    }
    
    // Print interfaces
    for _, iface := range resp.Payload.Interfaces {
        fmt.Printf("Interface: %s\n", iface.Interface)
        if iface.IPV4 != nil {
            fmt.Printf("  IPv4: %s\n", *iface.IPV4)
        }
        if iface.Mac != nil {
            fmt.Printf("  MAC: %s\n", *iface.Mac)
        }
        if iface.Vlan != nil {
            fmt.Printf("  VLAN: %d\n", *iface.Vlan)
        }
    }
}
```

## Python Client Example

```python
import json
import socket
import http.client

class UnixSocketHTTP:
    def __init__(self, socket_path):
        self.socket_path = socket_path
    
    def get(self, path):
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(self.socket_path)
        
        request = f"GET {path} HTTP/1.1\r\nHost: localhost\r\n\r\n"
        sock.sendall(request.encode())
        
        response = sock.recv(4096).decode()
        sock.close()
        
        # Parse response (simplified)
        headers, body = response.split('\r\n\r\n', 1)
        return json.loads(body)

# Query API
client = UnixSocketHTTP("/var/run/spidernet/spiderpool.sock")
data = client.get("/v1/workloadendpoint?podNamespace=default&podName=my-pod")

for iface in data.get('interfaces', []):
    print(f"Interface: {iface['interface']}")
    if 'mac' in iface:
        print(f"  MAC: {iface['mac']}")
    if 'vlan' in iface:
        print(f"  VLAN: {iface['vlan']}")
```

## Notes

- `mac` and `vlan` fields are **optional** - only present when provided during IP allocation
- Standard deployments without MAC assignment will not have these fields in the response
- Use optional field access in your client code
