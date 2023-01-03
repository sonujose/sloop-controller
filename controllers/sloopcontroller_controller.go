/*
Copyright 2022.

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
	"encoding/json"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kerr "k8s.io/apimachinery/pkg/api/errors"
	yaml2 "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	controllerv1 "sloop.io/ctrl/api/v1"
)

// SloopControllerReconciler reconciles a SloopController object
type SloopControllerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=controller.sloop.io,resources=sloopcontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controller.sloop.io,resources=sloopcontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controller.sloop.io,resources=sloopcontrollers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SloopController object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SloopControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Sloop configs...")

	var sloopController controllerv1.SloopController

	if err := r.Get(ctx, req.NamespacedName, &sloopController); err != nil {
		log.Error(err, "unable to fetch sloopController")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var sloopConfigSecrets corev1.SecretList
	if err := r.List(ctx, &sloopConfigSecrets, client.InNamespace(req.Namespace), client.MatchingLabels{"owner": "sloop", "status": "registered"}); err != nil {
		log.Error(err, "unable to list sloop config secrets.")
		return ctrl.Result{}, err
	}

	var sloopConfig SloopControllerConfig

	for _, configSecret := range sloopConfigSecrets.Items {

		err := json.Unmarshal(configSecret.Data["config"], &sloopConfig)

		if err != nil {
			log.Error(err, "Error encoding config to sloopConfig")
			continue
		}

		for _, comp := range sloopConfig.Config.Components {

			var resourceList []*unstructured.Unstructured

			log.Info("Sloop config data", "package", sloopConfig.Name, "revision", sloopConfig.Status.SyncRevision)

			for _, templateObj := range comp.TemplateFiles {

				manifestBlob := []byte(templateObj.ManifestYaml)
				u := &unstructured.Unstructured{}

				var decUnstructured = yaml2.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
				if _, _, err := decUnstructured.Decode(manifestBlob, nil, u); err != nil {
					log.Error(err, "Error parsing manifest blob to client.Object")
					continue
				}

				resourceList = append(resourceList, u)
			}

			r.ApplyResourceManifest(resourceList, log, sloopConfig, comp)
		}
	}

	return ctrl.Result{}, nil
}

func getlabelsForSloopManagedResource(revision string) map[string]string {

	sloopResourceLabels := map[string]string{
		"sloop.io/revision": revision,
	}

	return sloopResourceLabels
}

func (r *SloopControllerReconciler) ApplyResourceManifest(resourceList []*unstructured.Unstructured,
	log logr.Logger,
	sloopCfg SloopControllerConfig,
	comp Component) {

	var resourcesToCreate, resourcesToUpdate []*unstructured.Unstructured

	for _, res := range resourceList {

		res.SetLabels(getlabelsForSloopManagedResource("rev"))
		log.Info("Resource data", "blob", res.UnstructuredContent())
		log.Info("Trying to Parsed Resource data", "name", res.GetName(), "namespace", res.GetNamespace(), "kind", res.GetKind())

		existingresObj := &unstructured.Unstructured{}
		existingresObj.SetGroupVersionKind(res.GetObjectKind().GroupVersionKind())

		ctx := context.Background()

		err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: res.GetNamespace(),
			Name:      res.GetName()}, existingresObj)

		if kerr.IsNotFound(err) {
			resourcesToCreate = append(resourcesToCreate, res)
		} else {
			resourcesToUpdate = append(resourcesToUpdate, existingresObj)
		}
	}

	// Update Resources Dryrun
	err := r.CreateSloopResource(resourcesToCreate, true, log)
	if err != nil {
		log.Info("Dry run failed for resource create", "error", err)
		return
	}

	// Create Resources Dryrun
	err = r.UpdateSloopResource(resourcesToUpdate, true, log)
	if err != nil {
		log.Info("Dry run failed for resource updates", "error", err)
		return
	}

	log.Info("Dry run completed... Applying resources.")

	// Create Resources
	err = r.CreateSloopResource(resourcesToCreate, false, log)
	if err != nil {
		log.Info("Failed for resource create", "error", err)
		return
	}

	// Update Resources
	err = r.UpdateSloopResource(resourcesToUpdate, false, log)
	if err != nil {
		log.Info("Failed for resource updates", "error", err)
		return
	}

}

func (r *SloopControllerReconciler) UpdateSloopResource(resources []*unstructured.Unstructured, dryrun bool, log logr.Logger) error {

	var dryRunList []string

	if dryrun {
		dryRunList = append(dryRunList, "All")
	}

	for _, res := range resources {
		err := r.Client.Update(context.Background(), res, &client.UpdateOptions{DryRun: dryRunList})
		if err != nil {
			return err
		}

		if dryrun {
			log.Info("Dry run completed for resource updation", "name", res.GetName(), "namespace", res.GetNamespace(), "kind", res.GetKind())
			continue
		}
		log.Info("Success updating Resource", "name", res.GetName(), "namespace", res.GetNamespace(), "kind", res.GetKind())
	}
	return nil
}

func (r *SloopControllerReconciler) CreateSloopResource(resources []*unstructured.Unstructured, dryrun bool, log logr.Logger) error {

	var dryRunList []string

	if dryrun {
		dryRunList = append(dryRunList, "All")
	}

	for _, res := range resources {
		err := r.Client.Create(context.Background(), res, &client.CreateOptions{DryRun: dryRunList})
		if err != nil {
			return err
		}

		if dryrun {
			log.Info("Dry run completed for resource creation", "name", res.GetName(), "namespace", res.GetNamespace(), "kind", res.GetKind())
			continue
		}

		log.Info("Success Created Resource", "name", res.GetName(), "namespace", res.GetNamespace(), "kind", res.GetKind())
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SloopControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1.SloopController{}).
		Complete(r)
}
