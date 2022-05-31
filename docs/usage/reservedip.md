# Reserved IP

If any IP is not expected to be assigned to Pod, there is some solution

* ReservedIP CRD.

    ReservedIP takes effect global scope, it could prevent assigning IP from all ippool instances.

    It makes much sense to use ReservedIP like cases:

  * no matter how many ippool there is, or no matter what CIDR each ippool belong to, just set the reserved IP to ReservedIP CRD

* excludeIPs field in ippool CRD.

    excludeIPs field take effect just in its ippool, it just prevent assigning IP from local ippool.

    It makes much sense to use ReservedIP like following cases:

        ips: ["192.168.0.0/24"]
        excludeIPs: ["192.168.0.1","192.168.0.255"]
