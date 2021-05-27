package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	opsv1 "github.com/seanly/cluster-secret/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// SecretReconciler adds a secret to the default
// ServiceAccount in each namespace, unless the namespace
// has an ignore annotation
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const ignoreAnnotation = "k8ops.cn/cluster-secret.ignore"

func ignoredNamespace(ns *corev1.Namespace) bool {
	return ns.Annotations[ignoreAnnotation] == "1" || strings.ToLower(ns.Annotations[ignoreAnnotation]) == "true"
}

// Reconcile applies a number of ClusterPullSecrets to ServiceAccounts within
// various valid namespaces. Namespaces can be ignored as required.

func (r *SecretReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Log.WithValues("namespace", req.NamespacedName)

	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		r.Log.Info(fmt.Sprintf("%s", errors.Wrap(err, "unable to fetch secret")))
		return ctrl.Result{}, nil
	}

	clusterSecretList := &opsv1.ClusterSecretList{}
	err := r.Client.List(ctx, clusterSecretList)
	if err != nil {
		r.Log.Info(fmt.Sprintf("unable to list ClusterPullSecrets, %s", err.Error()))
		return ctrl.Result{}, nil
	}

	namespaces := &corev1.NamespaceList{}
	if err := r.Client.List(ctx, namespaces); err != nil {
		r.Log.Info(fmt.Sprintf("%s\n", errors.Wrap(err, "unable to fetch namespaces")))
	}

	r.Log.Info(fmt.Sprintf("Found %d namespaces", len(namespaces.Items)))

	for _, clusterSecret := range clusterSecretList.Items {

		if !clusterSecret.DeletionTimestamp.IsZero() {
			break
		}
		if clusterSecret.Spec.SecretRef.Namespace == secret.Namespace &&
			clusterSecret.Spec.SecretRef.Name == secret.Name {

			for _, namespace := range namespaces.Items {
				namespaceName := namespace.Name
				err := r.Reconcile2(clusterSecret, namespaceName)
				if err != nil {
					r.Log.Info(fmt.Sprintf("Found error: %s", err.Error()))
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) Reconcile2(clusterSecret opsv1.ClusterSecret, ns string) error {
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

	if clusterSecret.Spec.SecretRef == nil ||
		clusterSecret.Spec.SecretRef.Name == "" ||
		clusterSecret.Spec.SecretRef.Namespace == "" {
		return fmt.Errorf("no valid secretRef found on ClusterSecret: %s.%s",
			clusterSecret.Name,
			clusterSecret.Namespace)
	}

	seedSecret := &corev1.Secret{}
	if err := r.Get(ctx,
		client.ObjectKey{
			Name:      clusterSecret.Spec.SecretRef.Name,
			Namespace: clusterSecret.Spec.SecretRef.Namespace},
		seedSecret); err != nil {
		wrappedErr := errors.Wrapf(err, "unable to fetch seedSecret %s.%s", clusterSecret.Spec.SecretRef.Name, clusterSecret.Spec.SecretRef.Namespace)
		r.Log.Info(fmt.Sprintf("%s", wrappedErr.Error()))
		return wrappedErr
	}

	err := r.createSecret(clusterSecret, seedSecret, ns)
	if err != nil {
		r.Log.Info(err.Error())
		return err
	}

	if seedSecret.Type == corev1.SecretTypeDockerConfigJson {
		err = r.appendSecretToSA(clusterSecret, ns, "default")
		if err != nil {
			r.Log.Info(err.Error())
			return err
		}
	}

	return nil
}

func (r *SecretReconciler) listWithin(ns string) (*corev1.ServiceAccountList, error) {
	ctx := context.Background()
	SAs := &corev1.ServiceAccountList{}
	err := r.Client.List(ctx, SAs, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return SAs, nil
}

func (r *SecretReconciler) createSecret(clusterSecret opsv1.ClusterSecret, seedSecret *corev1.Secret, ns string) error {
	ctx := context.Background()

	secretKey := clusterSecret.Name

	nsSecret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: secretKey, Namespace: ns}, nsSecret)

	nsSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretKey,
			Namespace: ns,
			Annotations: map[string]string{
				"ops.k8ops.cn/cluster-secret": secretKey,
			},
		},
		Data: seedSecret.Data,
		Type: seedSecret.Type,
	}

	if err != nil {
		notFound := apierrors.IsNotFound(err)
		if notFound {
			err = ctrl.SetControllerReference(&clusterSecret, nsSecret, r.Scheme)
			if err != nil {
				r.Log.Info(fmt.Sprintf("can't create owner reference: %s.%s, %s", secretKey, ns, err.Error()))
			}
			err = r.Client.Create(ctx, nsSecret)
			if err != nil {
				r.Log.Info(fmt.Sprintf("can't create secret: %s.%s, %s", secretKey, ns, err.Error()))
				return err
			}
			r.Log.Info(fmt.Sprintf("created secret: %s.%s", secretKey, ns))
		}
	} else {
		err = ctrl.SetControllerReference(&clusterSecret, nsSecret, r.Scheme)
		if err != nil {
			r.Log.Info(fmt.Sprintf("can't create owner reference: %s.%s, %s", secretKey, ns, err.Error()))
		}

		err = r.Client.Update(ctx, nsSecret)
		if err != nil {
			r.Log.Info(fmt.Sprintf("can't update secret: %s.%s, %s", secretKey, ns, err.Error()))
			return err
		}
		r.Log.Info(fmt.Sprintf("update secret: %s.%s", secretKey, ns))
	}

	return nil
}

func (r *SecretReconciler) appendSecretToSA(clusterSecret opsv1.ClusterSecret, ns, serviceAccountName string) error {
	ctx := context.Background()

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

func hasImagePullSecret(sa *corev1.ServiceAccount, secretKey string) bool {
	found := false
	if len(sa.ImagePullSecrets) > 0 {
		for _, s := range sa.ImagePullSecrets {
			if s.Name == secretKey {
				found = true
				break
			}
		}
	}
	return found
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
