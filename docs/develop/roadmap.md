# roadmap

The following features are considered for the near future

## support CLI 

provide a CLI tool to debug and operate

* check which pod an IP is taken by 

* check IP usage

* trigger GC 

## support multi-cluster 

* when multi-cluster use Spiderpool to assign underlay IP address in the same CIDR, spiderpool could 
    synchronize ippool resource within a same subnet from other cluster, so it could help avoid IP conflict 

* when multi-cluster use Spiderpool to manager underlay IP address, leader cluster could
    synchronize all Spiderpool resource from member clusters, which help manager all underlay IP address

## integrate more CNI 

* integrate more CNI addon to solve underlay network needs 

## egress gateway for underlay solution 



