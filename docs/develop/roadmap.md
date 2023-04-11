# roadmap

The following features are considered for the near future

## support CLI 

provide a CLI tool to debug and operate

* check which pod an IP is taken by 

* check IP usage

* trigger GC 

## support none kubernetes-native controller

Spiderpool support to automatically manage ippool for application, it could create, delete, scale up and down a dedicated spiderippool object with static IP address just for one application.

This feature uses informer technology to watch application, parses its replicas number and manage spiderippool object, it works well with kubernetes-native controller like Deployment, ReplicaSet, StatefulSet, Job, CronJob, DaemonSet.

This feature also support none kubernetes-native controller, but Spiderpool could not parse the object yaml of none kubernetes-native controller, has some limitations: 

* does not support automatically scale up and down the IP

* does not support automatically delete the ippool

In the future, spiderpool may support all operation of automatical ippool.

## support multi-cluster 

* when multi-cluster use Spiderpool to assign underlay IP address in the same CIDR, spiderpool could 
    synchronize ippool resource within a same subnet from other cluster, so it could help avoid IP conflict 

* when multi-cluster use Spiderpool to manager underlay IP address, leader cluster could
    synchronize all Spiderpool resource from member clusters, which help manager all underlay IP address

## strengthen veth

* support to detect IP conflict when setting up the network of pod 

## integrate more CNI 

* integrate more CNI addon to solve underlay network needs 

## improve performance  

* continually improve the performance in kinds of scenes

