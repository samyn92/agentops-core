/*
Copyright 2026.

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

package v1alpha1

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var mcplog = logf.Log.WithName("mcpserver-webhook")

// SetupMCPServerWebhookWithManager registers the MCPServer validating webhook.
func (r *MCPServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-agents-agentops-io-v1alpha1-mcpserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=agents.agentops.io,resources=mcpservers,verbs=create;update,versions=v1alpha1,name=vmcpserver.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &MCPServer{}

// ValidateCreate implements webhook.CustomValidator.
func (r *MCPServer) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	mcplog.Info("validate create", "name", r.Name)
	mcp := obj.(*MCPServer)
	return mcp.validate()
}

// ValidateUpdate implements webhook.CustomValidator.
func (r *MCPServer) ValidateUpdate(_ context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	mcplog.Info("validate update", "name", r.Name)
	mcp := newObj.(*MCPServer)
	return mcp.validate()
}

// ValidateDelete implements webhook.CustomValidator.
func (r *MCPServer) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *MCPServer) validate() (admission.Warnings, error) {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	hasImage := r.Spec.Image != ""
	hasURL := r.Spec.URL != ""

	// Mutual exclusion: exactly one of image or url
	if hasImage && hasURL {
		allErrs = append(allErrs, field.Invalid(specPath.Child("image"), r.Spec.Image,
			"image and url are mutually exclusive; set exactly one"))
	}

	if !hasImage && !hasURL {
		allErrs = append(allErrs, field.Required(specPath.Child("image"),
			"one of image or url must be set"))
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "MCPServer"},
			r.Name, allErrs)
	}

	return nil, nil
}
