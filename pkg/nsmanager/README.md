# namespace manager

for spiderpool to get annotation for the default ippool of a namespace

```
type NsManager interface {
    GetDefaultIppool(nsName string)
    GetNamespaces()
}

func NewNsManager NsManager() {

}

```
