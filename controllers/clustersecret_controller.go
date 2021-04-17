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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	} else {

		r.Log.Info(fmt.Sprintf("Found: %s\n", clusterSecret.Name))

		namespaces := &corev1.NamespaceList{}
		if err := r.Client.List(ctx, namespaces); err != nil {
			r.Log.Info(fmt.Sprintf("%s\n", errors.Wrap(err, "unable to fetch namespaces")))
		}

		r.Log.Info(fmt.Sprintf("Found %d namespaces", len(namespaces.Items)))

		for _, namespace := range namespaces.Items {
			namespaceName := namespace.Name
			err := r.SecretReconciler.Reconcile(clusterSecret, namespaceName)
			if err != nil {
				r.Log.Info(fmt.Sprintf("Found error: %s", err.Error()))
			}
		}

	}

	return ctrl.Result{}, nil
}

func (r *ClusterSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1.ClusterSecret{}).
		Complete(r)
}
