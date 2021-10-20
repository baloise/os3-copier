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
	log := r.Log.WithValues("CopyResource", req.NamespacedName)

	copyResource := &resourcebaloisechv1alpha1.CopyResource{}
	err := r.Get(context.TODO(), req.NamespacedName, copyResource)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("CopyResource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get CopyResource.", "namespacedName", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	namespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      copyResource.Spec.MetaName,
	}

	sourceResource, _ := StringToStruct(copyResource.Spec.Kind)
	err = r.Client.Get(context.TODO(), namespacedName, sourceResource)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Source resource not found.", "namespacedName", namespacedName)
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "Source resource error.", "namespacedName", namespacedName)
		return ctrl.Result{}, nil
	}

	targetResource, _ := StringToStruct(copyResource.Spec.Kind)
	targetResource, _ = cloneResource(copyResource.Spec.Kind, sourceResource, targetResource)
	targetResource.SetResourceVersion("")
	targetResource.SetUID("")
	targetResource.SetNamespace(copyResource.Spec.TargetNamespace)
	targetResource.SetName(copyResource.Namespace + "-" + copyResource.Name)
	targetResource.SetOwnerReferences([]metav1.OwnerReference{buildOwnerReferenceToCopyResource(copyResource)})

	exists := isObjectExists(r, targetResource, log)

	if copyResource.Status.ResourceVersion == "" ||
		sourceResourceVersionHasChanged(copyResource.Spec.Kind, copyResource.Status.ResourceVersion, sourceResource) ||
		!exists {

		if !exists {
			err = r.Client.Create(context.TODO(), targetResource)
			if err != nil {
				log.Error(err, "Failed to create resource.", "name", targetResource.GetName(), "namespace ", targetResource.GetNamespace())
				return ctrl.Result{}, nil
			}
			log.Info("Successfully created.", "name", targetResource.GetName(), "namespace ", targetResource.GetNamespace())
		} else {
			err = r.Client.Update(context.TODO(), targetResource)
			if err != nil {
				log.Error(err, "Failed to update.", "name", targetResource.GetName(), "namespace ", targetResource.GetNamespace())
				return ctrl.Result{}, nil
			}
			log.Info("Successfully update.", "name", targetResource.GetName(), "namespace ", targetResource.GetNamespace())
		}

		copyResource.Status.ResourceVersion = getResourceVersion(copyResource.Spec.Kind, sourceResource)
		err := r.Status().Update(context.TODO(), copyResource)
		if err != nil {
			log.Error(err, "Failed to update CopyResource status.", "resourceVersion", copyResource.Status.ResourceVersion)
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *CopyResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcebaloisechv1alpha1.CopyResource{}).
		Complete(r)
}

func isObjectExists(r *CopyResourceReconciler, targetResource Object, log logr.Logger) bool {
	targetNamespacedName := types.NamespacedName{
		Namespace: targetResource.GetNamespace(),
		Name:      targetResource.GetName(),
	}
	// Use an unstructured type to avoid cache reader
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    targetResource.GetObjectKind().GroupVersionKind().Kind,
		Version: "v1",
	})
	err := r.Client.Get(context.TODO(), targetNamespacedName, u)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Not found " + targetNamespacedName.Namespace + "/" + targetNamespacedName.Name)
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
		return nil, fmt.Errorf("%s is not a known resource kind", kind)
	}
}

func cloneResource(kind string, source Object, target Object) (Object, error) {
	switch kind {
	case "Secret":
		sourceSecret := source.(*v1.Secret)
		copier.Copy(target.(*v1.Secret), sourceSecret)
		return target, nil
	case "ConfigMap":
		sourceConfigMap := source.(*v1.ConfigMap)
		copier.Copy(target.(*v1.ConfigMap), sourceConfigMap)
		return target, nil
	default:
		return nil, fmt.Errorf("%s is not a known resource kind", kind)
	}
}

func buildOwnerReferenceToCopyResource(copyResource *resourcebaloisechv1alpha1.CopyResource) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: copyResource.APIVersion,
		Kind:       copyResource.Kind,
		Name:       copyResource.GetName(),
		UID:        copyResource.GetUID(),
		// If true, this reference points to the managing controller.
		Controller: BoolPointer(true),
		// Don't block owner deletion (CopyResource) if this resource still exists
		BlockOwnerDeletion: BoolPointer(false),
	}
}

func sourceResourceVersionHasChanged(kind string, copyResourceVersion string, source Object) bool {
	sourceResourceVersion := getResourceVersion(kind, source)
	return sourceResourceVersion != copyResourceVersion
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

func BoolPointer(b bool) *bool {
	return &b
}
