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

// ServiceAccountWatcher reconciles a ServiceAccount object
type ServiceAccountWatcher struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/status,verbs=get;update;patch

func (r *ServiceAccountWatcher) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Log.WithValues("serviceaccount", req.NamespacedName)

	var sa corev1.ServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &sa); err != nil {
		r.Log.Info(fmt.Sprintf("%s", errors.Wrap(err, "unable to fetch serviceaccount")))
		return ctrl.Result{}, nil
	}
	
	if sa.Name != "default" {
		return ctrl.Result{}, nil
	}

	r.Log.Info(fmt.Sprintf("detected a change in serviceaccount: %s", sa.Name))
	
	clusterSecretList := &opsv1.ClusterSecretList{}
	err := r.Client.List(ctx, clusterSecretList)
	if err != nil {
		r.Log.Info(fmt.Sprintf("unable to list ClusterPullSecrets, %s", err.Error()))
		return ctrl.Result{}, nil
	}

	for _, clusterSecret := range clusterSecretList.Items {

		if !clusterSecret.DeletionTimestamp.IsZero() {
			break
		}

		seedSecret := &corev1.Secret{}
		if err := r.Get(ctx,
			client.ObjectKey{
				Name:      clusterSecret.Spec.SecretRef.Name,
				Namespace: clusterSecret.Spec.SecretRef.Namespace},
			seedSecret); err != nil {
			wrappedErr := errors.Wrapf(err, "unable to fetch seedSecret %s.%s", clusterSecret.Spec.SecretRef.Name, clusterSecret.Spec.SecretRef.Namespace)
			r.Log.Info(fmt.Sprintf("%s", wrappedErr.Error()))
			return ctrl.Result{}, nil
		}

		if seedSecret.Type == corev1.SecretTypeDockerConfigJson {

			err = r.appendSecretToSA(clusterSecret, sa.Namespace, sa.Name)
			if err != nil {
				r.Log.Info(err.Error())
				return ctrl.Result{}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ServiceAccountWatcher) appendSecretToSA(clusterSecret opsv1.ClusterSecret, ns, serviceAccountName string) error {
	ctx := context.Background()

	if !clusterSecret.DeletionTimestamp.IsZero() {
		return nil
	}

	secretKey := clusterSecret.Name

	sa := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: serviceAccountName, Namespace: ns}, sa)
	if err != nil {
		r.Log.Info(fmt.Sprintf("error getting SA in namespace: %s, %s", ns, err.Error()))
		wrappedErr := fmt.Errorf("unable to append pull secret to service account: %s", err)
		r.Log.Info(wrappedErr.Error())
		return wrappedErr
	}

	r.Log.Info(fmt.Sprintf("Pull secrets: %v", sa.ImagePullSecrets))

	hasSecret := hasImagePullSecret(sa, secretKey)

	if !hasSecret {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
			Name: secretKey,
		})

		err = r.Update(ctx, sa.DeepCopy())
		if err != nil {
			wrappedErr := fmt.Errorf("unable to append pull secret to service account: %s", err)
			r.Log.Info(wrappedErr.Error())
			return err
		}
	}

	return nil
}

func (r *ServiceAccountWatcher) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		Complete(r)
}
