# Performance

Continuous CI performance testing in e2e.

## Data interpretation

![CI performance](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/weizhoublue/38d00a872e830eedb46870c886549561/raw/spiderpoolperformance.json)

In the CI performance report, data is split into two groups with `|`.

The former represents the performance data of SpiderIPPool. The latter represents IPAM performance data for SpiderSubnet.

### SpiderIPPool

- The first data represents the time it takes for an application with 40 replicas to run from creation to run.

- The second data represents the time it takes for an application with 40 replicas to recover from rebuild to run.

- The third data represents the time it took for an application with 40 replicas to be completely deleted.

### SpiderSubnet

The second group separated by "|" is performance data based on SpiderSubnet assigning IP addresses to applications. The numbers correspond to creation, reconstruction, and deletion respectively. Its specifications and configuration are the same as SpiderIPPool mentioned above.
