# ipam manager

ipam in spiderpool agent

```

type IpamManager interface{
    Stop()
    AllocateIPS()
    ReleaseIPS()
    // for health check
    Health()
    // inform CNI the update information about pod ip
    RegisterWebHook( addr )
}

type ipammanager struct {
    metricManager
}

var _ IpamManager = &ipammanager{}

func NewIpamManager(metricManager) IpamManager {

}

func (t *ipammanager) AllocateIPS() {
}

func (t *ipammanager) ReleaseIPS() {
}

```
