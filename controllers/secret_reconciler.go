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
func (r *SecretReconciler) Reconcile(clusterSecret opsv1.ClusterSecret, ns string) error {
	ctx := context.Background()

	targetNS := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: ns}, targetNS); err != nil {
		wrappedErr := errors.Wrapf(err, "unable to fetch namespace: %s", ns)
		r.Log.Info(wrappedErr.Error())
		return wrappedErr
	}

	if ignoredNamespace(targetNS) {
		r.Log.Info(fmt.Sprintf("ignoring namespace %s due to annotation: %s ", ns, ignoreAnnotation))
		return nil
	}

	r.Log.Info(fmt.Sprintf("Getting SA for: %s", ns))

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

		SAs, err := r.listWithin(ns)
		if err != nil {
			wrappedErr := errors.Wrapf(err, "failed to list service accounts in %s namespace", ns)
			r.Log.Info(wrappedErr.Error())
			return wrappedErr
		}

		for _, sa := range SAs.Items {
			err = r.appendSecretToSA(clusterSecret, ns, sa.Name)
			if err != nil {
				r.Log.Info(err.Error())
				return err
			}
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

	secretKey := clusterSecret.Name + "-" + clusterSecret.Spec.Suffix

	nsSecret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: secretKey, Namespace: ns}, nsSecret)
	if err != nil {
		notFound := apierrors.IsNotFound(err)
		if !notFound {
			return errors.Wrap(err, "unexpected error checking for the namespaced pull secret")
		}

		r.Log.Info(fmt.Sprintf("secret not found: %s.%s, %s", secretKey, ns, err.Error()))

		nsSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretKey,
				Namespace: ns,
			},
			Data: seedSecret.Data,
			Type: seedSecret.Type,
		}

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

	return nil
}

func (r *SecretReconciler) appendSecretToSA(clusterSecret opsv1.ClusterSecret, ns, serviceAccountName string) error {
	ctx := context.Background()

	secretKey := clusterSecret.Name + "-" + clusterSecret.Spec.Suffix

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
