# e2e case for IPPool CR

| case id   | title                                                                                                      | priority | smoke | status | other |
|-----------|------------------------------------------------------------------------------------------------------------|----------|-------|--------|-------|
| D00001    | an ippool fails to add an IP that already exists in another ippool                                         | p2       |       | done   |       |
| D00002    | Successes to add correct ippool gateway and route etc. to a ippool CR                                      | p2       |       | NA     |       |
| D00003    | fails to add wrong ippool gateway and route to a ippool CR                                                 | p2       |       | NA     |       |
| D00004    | it fails to delete an ippool whose IP is not de-allocated at all                                           | p2       |       | NA     |       |
| D00005    | a "true" value of ippool/Spec/disabled should forbid IP allocation, but still allow ip de-allocation       | p2       |       | NA     |       |
| D00006    | succeed to create and delete IPPool in batches                                                             | p2       |       | NA     |       |
