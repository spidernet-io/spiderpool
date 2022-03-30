# namspace manager

for spiderpool to get annotation for ns default ippol

```
type NsManager interface {
    GetDefaultIppool(nsName string)
    GetNamespaces()
}

func NewNsManager NsManager() {

}

```
