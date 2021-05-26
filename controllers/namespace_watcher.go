package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	opsv1 "github.com/seanly/cluster-secret/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceWatcher watches namespaces for changes to
// trigger ClusterPullSecret reconciliation.
type NamespaceWatcher struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	SecretReconciler *SecretReconciler
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch

func (r *NamespaceWatcher) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Log.WithValues("namespace", req.NamespacedName)

	var namespace corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		r.Log.Info(fmt.Sprintf("%s", errors.Wrap(err, "unable to fetch clusterSecret")))
		return ctrl.Result{}, nil
	}

	r.Log.Info(fmt.Sprintf("detected a change in namespace: %s", namespace.Name))

	clusterSecretList := &opsv1.ClusterSecretList{}
	err := r.Client.List(ctx, clusterSecretList)
	if err != nil {
		r.Log.Info(fmt.Sprintf("unable to list ClusterSecrets, %s", err.Error()))
		return ctrl.Result{}, nil
	}

	for _, clusterSecret := range clusterSecretList.Items {
		if !clusterSecret.DeletionTimestamp.IsZero() {
			break
		}

		err := r.SecretReconciler.Reconcile(clusterSecret, namespace.Name)
		if err != nil {
			r.Log.Info(fmt.Sprintf("error reconciling namespace: %s with cluster pull secret: %s, error: %s",
				namespace.Name,
				clusterSecret.Name,
				err.Error()))
		}
	}

	return ctrl.Result{}, nil
}

func (r *NamespaceWatcher) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
