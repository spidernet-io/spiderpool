# Upgrading Spiderpool Versions

This document describes breaking changes, as well as how to fix them, that have occurred at given releases.
Please consult the segments from your current release until now before upgrading your spiderpool.

## Upgrade to 0.3.6 from (<=0.3.5)

### Description

1. There's a design flaw for SpiderSubnet feature in auto-created IPPool label.
   The previous label `ipam.spidernet.io/owner-application` corresponding value uses '-' as separative sign.
   For example, we have deployment `ns398-174835790/deploy398-82311862` and the corresponding label value is `Deployment-ns398-174835790-deploy398-82311862`.
   It's very hard to unpack it to trace back what the application namespace and name is.  
   Now, we use '_' rather than '-' as slash for SpiderSubnet feature label `ipam.spidernet.io/owner-application`, and the upper case
   will be like `Deployment_ns398-174835790_deploy398-82311862`.  
   Reference PR: [#1162](https://github.com/spidernet-io/spiderpool/pull/1162)
2. In order to support multiple interfaces with SpiderSubnet feature, we also add one more label for auto-created IPPool.
   The key is `ipam.spidernet.io/interface`, and the value is the corresponding interface name.

### Operation steps

1. Find all auto-created IPPools, their name format is `auto-${appKind}-${appNS}-${appName}-v${ipVersion}-${uid}` such as `auto-deployment-default-demo-deploy-subnet-v4-69d041b98b41`.

2. Replace their label, just like this:

   ```shell
    kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/owner-application": ${AppLabelValue}}}}'
   ```

3. Add one more label

   ```shell
    kubectl patch sp ${auto-pool} --type merge --patch '{"metadata": {"labels": {"ipam.spidernet.io/interface": "eth0"}}}}'
   ```

4. Update your Spiderpool components version and restart them all.

## Upgrade to 0.4.0 from (<0.4.0)

### Description

Due to the architecture adjustment, the SpiderEndpoint.Status.OwnerControllerType property is changed from `None` to `Pod`.

### Operation steps

1. Find all SpiderEndpoint objects that their Status OwnerControllerType is `None`

2. Replace the subresource SpiderEndpoint.Status.OwnerControllerType property from `None` to `Pod`
