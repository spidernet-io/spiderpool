# ippool CRD manager

Through informer, cache and manage ippool crd

```

type ippoolmanager struct {
    // when true, collect ippool metric for spiderpool-controller 
    CollectIppoolMetric bool
}

type IppoolManager interface {
    Run()
    Stop()
    GetPoolByName(name string)
    GetPools()
    UpdatePoolByName( .. ) (error)
    //for spiderpool-controller, validating webhook for ippool
    ValidatePoolModification(...)
    //for spiderpool-controller finalizer, check  whether there is ip still in use
    ValidatePoolDeletion(...)
}

func NewIppoolManager(CollectIppoolMetric ) IppoolManager {

}

```
