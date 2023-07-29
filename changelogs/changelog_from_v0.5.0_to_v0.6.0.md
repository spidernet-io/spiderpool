
# v0.6.0

***

## Feature

* add auto pool reclaim swicth support : [PR 1757](https://github.com/spidernet-io/spiderpool/pull/1757)

* add cluster subnet flexible ip number support : [PR 1763](https://github.com/spidernet-io/spiderpool/pull/1763)

* supplement annotation for Auto-Pool  : [PR 1768](https://github.com/spidernet-io/spiderpool/pull/1768)

* coordinator feature : [PR 1812](https://github.com/spidernet-io/spiderpool/pull/1812)

* support multiple default ippools : [PR 1914](https://github.com/spidernet-io/spiderpool/pull/1914)

* support multiple default ippools : [PR 1918](https://github.com/spidernet-io/spiderpool/pull/1918)

* multus cni config operator : [PR 1875](https://github.com/spidernet-io/spiderpool/pull/1875)

* feat: add cni plugin ifacer : [PR 1912](https://github.com/spidernet-io/spiderpool/pull/1912)

* default NIC could be specified by pod annotation : [PR 1934](https://github.com/spidernet-io/spiderpool/pull/1934)

* multus affinity : [PR 2003](https://github.com/spidernet-io/spiderpool/pull/2003)

* fix mutlus auto-generated cni files && opt coordinator cni config fields : [PR 2012](https://github.com/spidernet-io/spiderpool/pull/2012)



***

## Fix

* fix third-party controller auto-pool reclaim ippool symbol : [PR 1746](https://github.com/spidernet-io/spiderpool/pull/1746)

* fix Auto IPPool label value over maximum length limit : [PR 1749](https://github.com/spidernet-io/spiderpool/pull/1749)

* improve iprange function to reduce memory allocation : [PR 1776](https://github.com/spidernet-io/spiderpool/pull/1776)

* add ippool podAffinity validation : [PR 1806](https://github.com/spidernet-io/spiderpool/pull/1806)

* fix spiderpool-agent daemon data race : [PR 1839](https://github.com/spidernet-io/spiderpool/pull/1839)

* title:	fix spiderpool-agent daemon data race : [PR 1842](https://github.com/spidernet-io/spiderpool/pull/1842)

* fix coordinator disable failure : [PR 1856](https://github.com/spidernet-io/spiderpool/pull/1856)

* fix IP GC scan all running unhealthy : [PR 1883](https://github.com/spidernet-io/spiderpool/pull/1883)

* fix: binary absolute path for image command : [PR 1895](https://github.com/spidernet-io/spiderpool/pull/1895)

* title:	fix IP GC scan all running unhealthy : [PR 1894](https://github.com/spidernet-io/spiderpool/pull/1894)

* fix spiderpool-controller startup potential datarace : [PR 1909](https://github.com/spidernet-io/spiderpool/pull/1909)

* title:	fix spiderpool-controller startup potential datarace : [PR 1920](https://github.com/spidernet-io/spiderpool/pull/1920)

* fix coordinator spelling faults : [PR 1965](https://github.com/spidernet-io/spiderpool/pull/1965)

* fix multus-config CNI configuration master name faults and ifacer incorresponding configuration : [PR 1971](https://github.com/spidernet-io/spiderpool/pull/1971)

* slave must be down before creating bond : [PR 1972](https://github.com/spidernet-io/spiderpool/pull/1972)

* fix type assert faults : [PR 1979](https://github.com/spidernet-io/spiderpool/pull/1979)

* cleanup dirty rule、route、neigh tables in cmdAdd : [PR 2037](https://github.com/spidernet-io/spiderpool/pull/2037)

* fix failed to init arp client : [PR 2055](https://github.com/spidernet-io/spiderpool/pull/2055)

* implement IP allocation process document : [PR 2045](https://github.com/spidernet-io/spiderpool/pull/2045)

* coordinator: cmdDel should return nil : [PR 2059](https://github.com/spidernet-io/spiderpool/pull/2059)

* fix IPPool crd wrong vlanID range : [PR 2058](https://github.com/spidernet-io/spiderpool/pull/2058)

* ippool inherits subnet properties  : [PR 2036](https://github.com/spidernet-io/spiderpool/pull/2036)

* update CRD documentation : [PR 1904](https://github.com/spidernet-io/spiderpool/pull/1904)

* spidermultusconfig: improve parameters validate : [PR 2082](https://github.com/spidernet-io/spiderpool/pull/2082)

* ensure that the reply packet from the service is forwarded from veth0 : [PR 2078](https://github.com/spidernet-io/spiderpool/pull/2078)



***

## Totoal PR

[ 162 PR](https://github.com/spidernet-io/spiderpool/compare/v0.5.0...v0.6.0)
