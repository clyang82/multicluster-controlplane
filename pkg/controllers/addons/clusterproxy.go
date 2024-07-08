// Copyright Contributors to the Open Cluster Management project

package addons

import (
	"context"
	"fmt"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"open-cluster-management.io/cluster-proxy/pkg/proxyserver/controllers"
	"open-cluster-management.io/cluster-proxy/pkg/proxyserver/operator/authentication/selfsigned"
	ctrl "sigs.k8s.io/controller-runtime"
)

func SetupClusterProxyWithManager(ctx context.Context, mgr ctrl.Manager, kubeClient *kubernetes.Clientset,
	kubeInformers informers.SharedInformerFactory) error {

	// loading self-signer
	selfSigner, err := selfsigned.NewSelfSignerFromSecretOrGenerate(
		kubeClient, ProxyNamespace, "cluster-proxy-signer")
	if err != nil {
		return fmt.Errorf("failed loading self-signer: %v", err)
	}

	if err := controllers.RegisterClusterManagementAddonReconciler(
		mgr,
		selfSigner,
		kubeClient,
		kubeInformers.Core().V1().Secrets(),
		true,
	); err != nil {
		return fmt.Errorf("unable to create cluster proxy controller: %v", err)
	}

	return nil
}
