/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	opsv1 "github.com/seanly/cluster-secret/api/v1"
)

// ClusterSecretReconciler reconciles a ClusterSecret object
type ClusterSecretReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	SecretReconciler *SecretReconciler
}

// +kubebuilder:rbac:groups=ops.k8ops.cn,resources=clustersecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ops.k8ops.cn,resources=clustersecrets/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/status,verbs=get;update;patch

func (r *ClusterSecretReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("clustersecret", req.NamespacedName)

	var clusterSecret opsv1.ClusterSecret
	if err := r.Get(ctx, req.NamespacedName, &clusterSecret); err != nil {
		r.Log.Info(fmt.Sprintf("%s\n", errors.Wrap(err, "unable to fetch clusterSecret")))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: ???????????????webhook validate????????????CRD??????
	// ??????ClusterSecret????????????????????????SeedSecret??????
	if clusterSecret.Name == clusterSecret.Spec.SecretRef.Name {
		errorMsg := fmt.Sprintf("clusterSecret.name: %s, is equals .Spec.secretRef.name\n", clusterSecret.Name)
		r.Log.Info(errorMsg)
		return ctrl.Result{}, fmt.Errorf(errorMsg)
	}

	r.Log.Info(fmt.Sprintf("Found ClusterSecret: %s\n", clusterSecret.Name))

	// ??????ClusterSecret??????
	defer func() {
		if err := r.Update(ctx, &clusterSecret); err != nil {
			r.Log.Error(err, "patch failed")
		}
	}()

	// ????????? Finalizers ????????? k8s ??? delete ?????????????????? metadata.deletionTimestamp ??????
	if !clusterSecret.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(&clusterSecret)
	}
	// ?????????0 ????????????????????????????????????????????????????????? finalizer????????????????????????????????????????????????????????????
	return r.reconcileNormal(&clusterSecret)
}

func (r *ClusterSecretReconciler) reconcileDelete(clusterSecret *opsv1.ClusterSecret) (ctrl.Result, error) {
	// ???????????? 0 ???????????????????????????
	// ???????????? finalizer ????????????????????? finalizer ??????????????????????????? hook ??????
	if err := r.deleteClusterSecretResources(clusterSecret); err != nil {
		return ctrl.Result{}, err
	}

	// ???????????? hook ??????????????????????????? finalizers??? k8s ??????????????????
	ctrlutil.RemoveFinalizer(clusterSecret, opsv1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *ClusterSecretReconciler) reconcileNormal(clusterSecret *opsv1.ClusterSecret) (ctrl.Result, error) {

	ctx := context.Background()
	ctrlutil.AddFinalizer(clusterSecret, opsv1.ClusterFinalizer)

	// ????????????????????????
	namespaces := &corev1.NamespaceList{}
	if err := r.Client.List(ctx, namespaces); err != nil {
		r.Log.Info(fmt.Sprintf("%s\n", errors.Wrap(err, "unable to fetch namespaces")))
	}

	r.Log.Info(fmt.Sprintf("Found %d namespaces", len(namespaces.Items)))

	for _, namespace := range namespaces.Items {
		namespaceName := namespace.Name
		err := r.SecretReconciler.Reconcile2(*clusterSecret, namespaceName)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Found error: %s", err.Error()))
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterSecretReconciler) deleteClusterSecretResources(clusterSecret *opsv1.ClusterSecret) error {
	//
	// ?????? ClusterSecret???????????????????????????
	//
	ctx := context.Background()
	namespaces := &corev1.NamespaceList{}
	if err := r.Client.List(ctx, namespaces); err != nil {
		r.Log.Info(fmt.Sprintf("%s\n", errors.Wrap(err, "unable to fetch namespaces")))
	}
	r.Log.Info(fmt.Sprintf("Found %d namespaces", len(namespaces.Items)))

	for _, namespace := range namespaces.Items {
		namespaceName := namespace.Name
		err := r.deleteClusterSecret(clusterSecret, namespaceName)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Found error: %s", err.Error()))
			return err
		}
	}

	return nil
}

func (r *ClusterSecretReconciler) deleteClusterSecret(clusterSecret *opsv1.ClusterSecret, ns string) error {

	ctx := context.Background()

	targetNS := &corev1.Namespace{}

	if err := r.Get(ctx, client.ObjectKey{Name: ns}, targetNS); err != nil {
		wrappedErr := errors.Wrapf(err, "unable to fetch namespace: %s", ns)
		r.Log.Info(wrappedErr.Error())
		return wrappedErr
	}

	if len(clusterSecret.Spec.Namespaces) > 0 {
		if _, exist := Find(clusterSecret.Spec.Namespaces, targetNS.Name); !exist {
			return nil
		}
	}

	if ignoredNamespace(targetNS) {
		r.Log.Info(fmt.Sprintf("ignoring namespace %s due to annotation: %s ", ns, ignoreAnnotation))
		return nil
	}

	secrets := &corev1.SecretList{}
	if err := r.Client.List(ctx, secrets, client.InNamespace(ns)); err != nil {
		return client.IgnoreNotFound(err)
	}

	for _, secret := range secrets.Items {

		// ??????annotation 'ops.k8ops.cn/cluster-secret': 'ClusterSecret.name'
		if clusterSecret.Name == secret.Annotations["ops.k8ops.cn/cluster-secret"] &&
			secret.Name == clusterSecret.Name {

			// ?????? ServiceAccount ?????????ImagePullSecret??????
			if err := r.cleanPullSecretInSA(secret, ns); err != nil {
				return err
			}

			r.Log.Info(fmt.Sprintf("delete secret : %s, in namespace(%s)", secret.Name, ns))
			if err := r.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ClusterSecretReconciler) cleanPullSecretInSA(secret corev1.Secret, ns string) error {

	ctx := context.Background()

	sa := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: "default", Namespace: ns}, sa)
	if err != nil {
		r.Log.Info(fmt.Sprintf("error getting SA in namespace: %s, %s", ns, err.Error()))
		wrappedErr := fmt.Errorf("unable to append pull secret to service account: %s", err)
		r.Log.Info(wrappedErr.Error())
		return wrappedErr
	}

	for i, pullSecret := range sa.ImagePullSecrets {
		if pullSecret.Name == secret.Name {
			sa.ImagePullSecrets = append(sa.ImagePullSecrets[:i], sa.ImagePullSecrets[i+1:]...)
			break
		}
	}

	if err = r.Update(ctx, sa.DeepCopy()); err != nil {
		wrappedErr := fmt.Errorf("unable to append pull secret to service account: %s", err)
		r.Log.Info(wrappedErr.Error())
		return err
	}

	return nil
}

func (r *ClusterSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1.ClusterSecret{}).
		Complete(r)
}
