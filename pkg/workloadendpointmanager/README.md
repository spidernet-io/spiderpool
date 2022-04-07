# workloadendpoint CRD manager

cache all workloadendpoint who locates on local host

```

type WorkloadEndpointManager interface {
    Run()
    Stop()
    GetWorkloadEndpointByName(name string)
    GetWorkloadEndpoints()
    DeleteWorkloadEndpointByName(name string)
    UpdateWorkloadEndpointByName( .. ) (error)
}

func NewWorkloadEndpointManager( ) WorkloadEndpointManager {

}

```
