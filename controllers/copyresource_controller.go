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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcebaloisechv1alpha1 "github.com/baloise/os3-copier/api/v1alpha1"
	"github.com/jinzhu/copier"
)

// CopyResourceReconciler reconciles a CopyResource object
type CopyResourceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type Object interface {
	metav1.Object
	runtime.Object
}

// +kubebuilder:rbac:groups=resource.baloise.ch.baloise.ch,resources=copyresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=resource.baloise.ch.baloise.ch,resources=copyresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=resource.baloise.ch.baloise.ch,resources=copyresources/finalizers,verbs=update
// +kubebuilder:rbac:groups=v1,resources=secret,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=v1,resources=configmap,verbs=get;list;watch;create;update;patch;delete

func (r *CopyResourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	//_ = r.Log.WithValues("copyresource", req.NamespacedName)
	log := r.Log.WithValues("copyresource", req.NamespacedName)

	copyResource := &resourcebaloisechv1alpha1.CopyResource{}
	err := r.Get(ctx, req.NamespacedName, copyResource)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("CopyResource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get CopyResource.")
		return ctrl.Result{}, err
	}

	namespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      copyResource.Spec.MetaName,
	}

	sourceResource, _ := StringToStruct(copyResource.Spec.Kind)

	err = r.Client.Get(ctx, namespacedName, sourceResource)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "Get Resource error.")
		return ctrl.Result{Requeue: true}, nil
	}

	targetResource, _ := StringToStruct(copyResource.Spec.Kind)
	targetResource, _ = cloneResource(copyResource.Spec.Kind, sourceResource, targetResource)

	targetResource.SetResourceVersion("")
	targetResource.SetUID("")
	targetResource.SetNamespace(copyResource.Spec.TargetNamespace)
	targetResource.SetName(copyResource.Namespace + "-" + copyResource.Name)

	targetNamespacedName := types.NamespacedName{
		Namespace: targetResource.GetNamespace(),
		Name:      targetResource.GetName(),
	}
	//targetNamespacedObject, _ := StringToStruct(copyResource.Spec.Kind)
	err = r.Client.Get(ctx, targetNamespacedName, targetResource)

	if copyResource.Status.ResourceVersion == "" || sourceResourceVersionHasChanged(copyResource.Spec.Kind, copyResource.Status.ResourceVersion, sourceResource) || errors.IsNotFound(err) {
		err = r.Client.Create(ctx, targetResource)
		if err != nil && errors.IsAlreadyExists(err) {
			err = r.Client.Update(ctx, targetResource)
		}
		if err == nil {
			copyResource.Status.ResourceVersion = getResourceVersion(copyResource.Spec.Kind, sourceResource)
			err := r.Status().Update(ctx, copyResource)
			if err != nil {
				log.Error(err, "Failed to update CopyResource status")
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func StringToStruct(kind string) (Object, error) {
	switch kind {
	case "Secret":
		return &v1.Secret{}, nil
	case "ConfigMap":
		return &v1.ConfigMap{}, nil
	default:
		return nil, fmt.Errorf("%s is not a known resource name", kind)
	}
}

func cloneResource(kind string, source Object, target Object) (Object, error) {
	switch kind {
	case "Secret":
		sourceSecret := source.(*v1.Secret)
		copier.Copy(target.(*v1.Secret), sourceSecret)
		target.SetResourceVersion(sourceSecret.ResourceVersion)
		return target, nil
	case "ConfigMap":
		sourceConfigMap := source.(*v1.ConfigMap)
		copier.Copy(target.(*v1.ConfigMap), sourceConfigMap)
		target.SetResourceVersion(sourceConfigMap.ResourceVersion)
		return target, nil
	default:
		return nil, fmt.Errorf("%s is not a known resource name", kind)
	}
}

func sourceResourceVersionHasChanged(kind string, copyRessourceVersion string, source Object) bool {
	sourceResourceVersion := getResourceVersion(kind, source)
	return sourceResourceVersion != copyRessourceVersion
}

func getResourceVersion(kind string, resource Object) string {
	switch kind {
	case "Secret":
		return resource.(*v1.Secret).ResourceVersion
	case "ConfigMap":
		return resource.(*v1.ConfigMap).ResourceVersion
	default:
		return ""
	}
}

func (r *CopyResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcebaloisechv1alpha1.CopyResource{}).
		Complete(r)
}
