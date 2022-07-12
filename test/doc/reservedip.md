# e2e case for reservedip

| case id | title                                                                            | priority | smoke | status | other |
|---------|----------------------------------------------------------------------------------|----------|-------|--------|-------|
| S00001  | an IP who is set in ReservedIP CRD, should not be assigned to a pod              | p2       |       | NA     |       |
| S00002  | an IP who is set in excludeIPs field of IPPool, should not be assigned to a pod  | p2       |       | done   |       |
| S00003  | Failed to set same IP in excludeIPs when an IP is assigned to a pod              | p2       |       | NA     |       |
