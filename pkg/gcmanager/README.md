# Spiderpool IP garbage collection

## Notice

* Current Tracing mechanism will not trace 'Terminating' time out pod if it's a StatefulSet pod, and the StatefulSet object was deleted or decreased its replicas.
