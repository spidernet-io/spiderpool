// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	spiderpoolv1types "github.com/spidernet-io/spiderpool/pkg/types"
)

const (
	EnvDefaultIPv4PoolName     = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_NAME"
	EnvDefaultIPv4PoolSubnet   = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_SUBNET"
	EnvDefaultIPv4PoolIPRanges = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_IPRANGES"

	EnvDefaultIPv6PoolName     = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_NAME"
	EnvDefaultIPv6PoolSubnet   = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_SUBNET"
	EnvDefaultIPv6PoolIPRanges = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_IPRANGES"
)

var (
	scheme = runtime.NewScheme()

	logger = logutils.Logger.Named("Default-IPPool-Installation")

	maxFailedRetries = 5
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv1.AddToScheme(scheme))
}

func main() {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*10)
	defer cancel()

	runtimeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if nil != err {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// validate IPv4 default pool params
	defaultV4PoolName, defaultV4PoolSubnet, defaultV4PoolIPRanges, err := validatePoolConfigs(EnvDefaultIPv4PoolName, EnvDefaultIPv4PoolSubnet, EnvDefaultIPv4PoolIPRanges, 4)
	if nil != err {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// validate IPv6 default pool params
	defaultV6PoolName, defaultV6PoolSubnet, defaultV6PoolIPRanges, err := validatePoolConfigs(EnvDefaultIPv6PoolName, EnvDefaultIPv6PoolSubnet, EnvDefaultIPv6PoolIPRanges, 6)
	if nil != err {
		logger.Error(err.Error())
		os.Exit(1)
	}

	if len(defaultV4PoolName) != 0 {
		logger.Sugar().Infof("try to create SpiderIPPool '%s', subent:'%s', IP ranges: '%v'",
			defaultV4PoolName, defaultV4PoolSubnet, defaultV4PoolIPRanges)

		err = createDefaultPoolIfNotExist(ctx, runtimeClient, defaultV4PoolName, defaultV4PoolSubnet, defaultV4PoolIPRanges)
		if nil != err {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}

	if len(defaultV6PoolName) != 0 {
		logger.Sugar().Infof("try to create SpiderIPPool '%s', subent:'%s', IP ranges: '%v'",
			defaultV6PoolName, defaultV6PoolSubnet, defaultV6PoolIPRanges)

		err = createDefaultPoolIfNotExist(ctx, runtimeClient, defaultV6PoolName, defaultV6PoolSubnet, defaultV6PoolIPRanges)
		if nil != err {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}
}

func validatePoolConfigs(envPoolName, envPoolSubnet, envPoolIPRanges string, ipVersion spiderpoolv1types.IPVersion) (poolName, poolSubnet string, poolIPRanges []string, err error) {
	defaultPoolName := os.Getenv(envPoolName)
	defaultPoolSubnet := os.Getenv(envPoolSubnet)
	defaultPoolIPRangesStr := os.Getenv(envPoolIPRanges)

	if len(defaultPoolName) != 0 {
		if len(defaultPoolSubnet) == 0 {
			return "", "", nil, fmt.Errorf("SpiderIPPool '%s' environment variable '%s' must be specified", defaultPoolName, envPoolSubnet)
		}

		var defaultPoolIPRanges []string
		err = json.Unmarshal([]byte(defaultPoolIPRangesStr), &defaultPoolIPRanges)
		if nil != err {
			return "", "", nil, fmt.Errorf("failed to parse SpiderIPPool '%s' IPs '%s', error: %v", defaultPoolName, defaultPoolIPRangesStr, err)
		}

		_, err = spiderpoolip.ParseIPRanges(ipVersion, defaultPoolIPRanges)
		if nil != err {
			return "", "", nil, err
		}
	}

	return "", "", nil, nil
}

func createDefaultPoolIfNotExist(ctx context.Context, runtimeClient client.Client, defaultPoolName, defaultPoolSubnet string, defaultPoolIPRanges []string) error {
	// check the pool whether is existed or not
	poolExist, err := isPoolExist(ctx, runtimeClient, defaultPoolName)
	if nil != err {
		return err
	}

	// create default pool if not exist
	if !poolExist {
		err = createPoolWithConfig(ctx, runtimeClient, defaultPoolName, defaultPoolSubnet, defaultPoolIPRanges)
		if nil != err {
			return err
		}
	} else {
		logger.Sugar().Infof("SpiderIPPool '%s' already exists, no need to create it again", defaultPoolName)
	}

	return nil
}

// isPoolExist checks whether the SpiderIPPool object exists or not with the given pool name.
func isPoolExist(ctx context.Context, runtimeClient client.Client, poolName string) (bool, error) {
	for i := 0; i <= maxFailedRetries; i++ {
		var ipPool spiderpoolv1.SpiderIPPool
		err := runtimeClient.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool)
		if nil != err {
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			if i == maxFailedRetries {
				return false, fmt.Errorf("failed to get Spiderpool '%s', error: %v", poolName, err)
			}

			time.Sleep(time.Second * 5)
			continue
		}

		return true, nil
	}

	return false, nil
}

// createPoolWithConfig will create SpiderIPPool object with the given params
func createPoolWithConfig(ctx context.Context, runtimeClient client.Client, poolName, subnet string, ipRanges []string) error {
	for i := 0; i <= maxFailedRetries; i++ {
		pool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: poolName},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet: subnet,
				IPs:    ipRanges,
			},
		}

		err := runtimeClient.Create(ctx, pool)
		if nil != err {
			if i == maxFailedRetries {
				return err
			}

			time.Sleep(time.Second * 5)
			continue
		}

		logger.Sugar().Infof("create SpiderIPPool '%+v' successfully", *pool)
		return nil
	}

	return fmt.Errorf("failed to create SpiderIPPool '%s'", poolName)
}
