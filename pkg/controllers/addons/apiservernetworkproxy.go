// Copyright Contributors to the Open Cluster Management project

package addons

import (
	"context"
	"fmt"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/apiserver-network-proxy/cmd/server/app"
	"sigs.k8s.io/apiserver-network-proxy/cmd/server/app/options"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	signingAgentServerSecretName    = "agent-server"
	signingProxyServerSecretName    = "proxy-server"
	siginingProxyServerCaSecretName = "proxy-server-ca"
	ProxyNamespace                  = "cluster-proxy"
	ProxySecretsLocation            = "/etc/cluster-proxy/proxy"
	AgentSecretsLocation            = "/etc/cluster-proxy/agent"
)

func SetupAPIServerNetworkProxyWithManager(ctx context.Context, mgr ctrl.Manager, kubeClient *kubernetes.Clientset) error {
	reconciler := &APIServerNetworkProxyReconciler{
		client: kubeClient,
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &APIServerNetworkProxyReconciler{}

type APIServerNetworkProxyReconciler struct {
	client *kubernetes.Clientset
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServerNetworkProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			&corev1.Secret{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return e.Object.GetName() == signingAgentServerSecretName ||
						e.Object.GetName() == signingProxyServerSecretName ||
						e.Object.GetName() == siginingProxyServerCaSecretName
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					if e.ObjectNew.GetName() == signingAgentServerSecretName ||
						e.ObjectNew.GetName() == signingProxyServerSecretName ||
						e.ObjectNew.GetName() == siginingProxyServerCaSecretName {
						newSecret := e.ObjectNew.(*corev1.Secret)
						oldSecret := e.ObjectOld.(*corev1.Secret)
						// only enqueue the obj when secret data changed
						return !reflect.DeepEqual(newSecret.Data, oldSecret.Data)
					}
					return false
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return e.Object.GetName() == signingAgentServerSecretName ||
						e.Object.GetName() == signingProxyServerSecretName ||
						e.Object.GetName() == siginingProxyServerCaSecretName
				},
			}),
		).Complete(r)
}

func (r *APIServerNetworkProxyReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Start reconciling")

	agentServerSecret, err := r.client.CoreV1().Secrets(ProxyNamespace).Get(ctx, signingAgentServerSecretName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to get agent server secret")
		return reconcile.Result{}, err
	}

	proxyServerSecret, err := r.client.CoreV1().Secrets(ProxyNamespace).Get(ctx, signingProxyServerSecretName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to get proxy server secret")
		return reconcile.Result{}, err
	}

	proxyServerCaSecret, err := r.client.CoreV1().Secrets(ProxyNamespace).Get(ctx, siginingProxyServerCaSecretName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed to get proxy server ca secret")
		return reconcile.Result{}, err
	}

	if err = persistSecretData(agentServerSecret, AgentSecretsLocation); err != nil {
		logger.Error(err, "Failed to persist agent server secret data")
		return reconcile.Result{}, err
	}
	if err = persistSecretData(proxyServerSecret, ProxySecretsLocation); err != nil {
		logger.Error(err, "Failed to persist proxy server secret data")
		return reconcile.Result{}, err
	}
	if err = persistSecretData(proxyServerCaSecret, ProxySecretsLocation); err != nil {
		logger.Error(err, "Failed to persist proxy server ca secret data")
		return reconcile.Result{}, err
	}

	proxy := &app.Proxy{}
	o := options.NewProxyRunOptions()
	o.ClusterCaCert = fmt.Sprintf("%s/ca.crt", ProxySecretsLocation)
	o.ClusterCert = fmt.Sprintf("%s/tls.crt", ProxySecretsLocation)
	o.ClusterKey = fmt.Sprintf("%s/tls.key", ProxySecretsLocation)
	o.ServerCaCert = fmt.Sprintf("%s/ca.crt", ProxySecretsLocation)
	o.ServerCert = fmt.Sprintf("%s/tls.crt", AgentSecretsLocation)
	o.ServerKey = fmt.Sprintf("%s/tls.key", AgentSecretsLocation)
	o.ProxyStrategies = "destHost"
	o.ServerCount = 1

	if err = proxy.Run(o, ctx.Done()); err != nil {
		logger.Error(err, "Failed to run proxy server")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func persistSecretData(secret *corev1.Secret, location string) error {
	if secret.Data == nil {
		return fmt.Errorf("secret data is nil")
	}
	for k, v := range secret.Data {
		err := os.WriteFile(fmt.Sprintf("%s/%s", location, k), v, 0420)
		if err != nil {
			return err
		}
	}
	return nil
}
