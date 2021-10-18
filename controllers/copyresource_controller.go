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
	"github.com/prometheus/common/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

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

// +kubebuilder:rbac:groups=resource.baloise.ch,resources=copyresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=resource.baloise.ch,resources=copyresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=resource.baloise.ch,resources=copyresources/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=secrets/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=configmaps/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=configmaps/status,verbs=get;update;patch

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
		return ctrl.Result{}, nil
	}

	targetResource, _ := StringToStruct(copyResource.Spec.Kind)
	targetResource, _ = cloneResource(copyResource.Spec.Kind, sourceResource, targetResource)
	targetResource.SetResourceVersion("")
	targetResource.SetUID("")
	targetResource.SetNamespace(copyResource.Spec.TargetNamespace)
	targetResource.SetName(copyResource.Namespace + "-" + copyResource.Name)
	targetResource.SetOwnerReferences(nil)
	ownerReference := buildOwnerReferenceToCopyRessource(copyResource)
	targetResource.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	exists := existsInTargetNamespace(r, targetResource)

	if copyResource.Status.ResourceVersion == "" ||
		sourceResourceVersionHasChanged(copyResource.Spec.Kind, copyResource.Status.ResourceVersion, sourceResource) ||
		!exists {

		if !exists {
			err = r.Client.Create(ctx, targetResource)
			if err != nil {
				log.Error(err, "Failed to create "+targetResource.GetName()+" in namesapce "+targetResource.GetNamespace())
				return ctrl.Result{}, err
			}
			log.Info("successfully created " + targetResource.GetName() + " in namesapce " + targetResource.GetNamespace())
		} else {
			err = r.Client.Update(ctx, targetResource)
			if err == nil {
				log.Info("successfully update " + targetResource.GetName() + " in namesapce " + targetResource.GetNamespace())
			} else {
				log.Error(err, "Failed to update "+targetResource.GetName()+" in namesapce "+targetResource.GetNamespace())
				return ctrl.Result{}, err
			}
		}

		copyResource.Status.ResourceVersion = getResourceVersion(copyResource.Spec.Kind, sourceResource)
		err := r.Status().Update(ctx, copyResource)
		if err != nil {
			log.Error(err, "Failed to update CopyResource status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func existsInTargetNamespace(r *CopyResourceReconciler, targetResource Object) bool {
	targetNamespacedName := types.NamespacedName{
		Namespace: targetResource.GetNamespace(),
		Name:      targetResource.GetName(),
	}
	// use an unstructured type for the search because Get uses a cached reader for structured type and the ressource could not be found.
	// maybe because we try to read it from a different namespace.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    targetResource.GetObjectKind().GroupVersionKind().Kind,
		Version: "v1",
	})
	err := r.Client.Get(context.TODO(), targetNamespacedName, u)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Failed to search for target ressource:", targetNamespacedName)
		}
		return false
	}
	return true
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

func buildOwnerReferenceToCopyRessource(copyResource *resourcebaloisechv1alpha1.CopyResource) metav1.OwnerReference {
	ownerReference := metav1.OwnerReference{}
	ownerReference.APIVersion = copyResource.APIVersion
	ownerReference.Kind = copyResource.Kind
	ownerReference.Name = copyResource.GetName()
	ownerReference.UID = copyResource.GetUID()
	// If true, this reference points to the managing controller.
	ownerReference.Controller = refToBoolTrue()
	// If true, AND if the owner has the "foregroundDeletion" finalizer, then
	// the owner cannot be deleted from the key-value store until this
	// reference is removed.
	// Defaults to false.
	// To set this field, a user needs "delete" permission of the owner,
	// otherwise 422 (Unprocessable Entity) will be returned.
	ownerReference.BlockOwnerDeletion = refToBoolFalse()
	return ownerReference
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

func refToBoolFalse() *bool {
	b := false
	return &b
}

func refToBoolTrue() *bool {
	b := true
	return &b
}
