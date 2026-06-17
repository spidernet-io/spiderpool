# Contract: Kubelet Plugin Paths

## Configuration

`spiderpoolAgent.networkResourcePlugin.kubeletRootDir` defines the kubelet root directory used to derive agent hostPath mounts and runtime selection.

Default:

```text
/var/lib/kubelet
```

## Derived Paths

- Device plugin path: `{kubeletRootDir}/device-plugins`
- Plugin registration path: `{kubeletRootDir}/plugins_registry`

## Mount Contract

When `spiderpoolAgent.networkResourcePlugin.enabled=true`, spiderpool-agent must mount both derived host paths read-write.

Example with the default root:

```yaml
- name: device-plugin
  mountPath: /var/lib/kubelet/device-plugins
  readOnly: false
- name: plugins-registry
  mountPath: /var/lib/kubelet/plugins_registry
  readOnly: false
```

## Selection Contract

Runtime selection order:

1. Use `{kubeletRootDir}/plugins_registry` when it exists.
2. Use `{kubeletRootDir}/device-plugins` only when the preferred directory is absent.
3. Emit diagnostics with the selected path and fallback reason.

## Validation Contract

- `kubeletRootDir` must be absolute.
- Rendering tests must verify both mounts for default and non-default roots.
- Unit tests must verify preference and fallback behavior.
