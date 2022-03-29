# gc

gc ip for spiderpool-controller

```

type gcmanager struct {
    metricManager
}

type GcManager interface {
    Stop()
    // for CLI to debug
    GetPodDatabase()
    // for CLI to trigger GC all
    TriggerGcAll()
    // for health check
    Health()
}

func NewGcManager( gcConfig ...  , metricManager ) GcManager {
    
}

```
