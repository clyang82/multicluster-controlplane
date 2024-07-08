// Copyright Contributors to the Open Cluster Management project

package addons

import (
	"context"
	"sync/atomic"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"open-cluster-management.io/cluster-proxy/pkg/common"
	"open-cluster-management.io/cluster-proxy/pkg/util"
)

func StartLocalProxy(ctx context.Context, hubKubeConfig *rest.Config) error {
	klog.Infof("Running local port-forward proxy")
	readiness := &atomic.Value{}
	readiness.Store(true)
	rr := util.NewRoundRobinLocalProxy(
		hubKubeConfig,
		readiness,
		"cluster-proxy",
		common.LabelKeyComponentName+"="+common.ComponentNameProxyServer,
		8091,
	)
	_, err := rr.Listen(ctx)
	if err != nil {
		return err
	}
	return nil
}
