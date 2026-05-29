/*
Copyright 2026 Xavier Portilla Edo.

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

package controller

import (
	"context"
	"encoding/json"
	"strings"

	"sigs.k8s.io/yaml"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	genkitv1alpha1 "github.com/xavidop/genkit-operator/api/v1alpha1"
)

// PromptReconciler reconciles Prompt resources. It validates content,
// computes its SHA-256 content hash, parses the dotprompt frontmatter on
// a best-effort basis, and reflects all of that in status.
type PromptReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=genkit.dev,resources=prompts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=genkit.dev,resources=prompts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=genkit.dev,resources=prompts/finalizers,verbs=update

// Reconcile validates spec.content and refreshes content hash & parsed
// frontmatter in status.
func (r *PromptReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var p genkitv1alpha1.Prompt
	if err := r.Get(ctx, req.NamespacedName, &p); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := p.DeepCopy()
	p.Status.ObservedGeneration = p.Generation

	if strings.TrimSpace(p.Spec.Content) == "" {
		setCondition(&p.Status.Conditions, notReadyCondition(
			p.Generation, genkitv1alpha1.ReasonInvalidSpec,
			"spec.content is empty",
		))
		return r.patchStatus(ctx, original, &p)
	}

	p.Status.ContentHash = sha256Hex([]byte(p.Spec.Content))
	p.Status.ParsedFrontmatter = parseDotpromptFrontmatter(p.Spec.Content)

	setCondition(&p.Status.Conditions, readyCondition(
		p.Generation,
		"prompt content parsed",
	))
	return r.patchStatus(ctx, original, &p)
}

// parseDotpromptFrontmatter extracts the YAML frontmatter block delimited
// by leading `---` and a trailing `---`. Returns nil on any parse error.
func parseDotpromptFrontmatter(content string) *apiextensionsv1.JSON {
	const sep = "---"
	trimmed := strings.TrimLeft(content, "\n\r ")
	if !strings.HasPrefix(trimmed, sep) {
		return nil
	}
	rest := strings.TrimPrefix(trimmed, sep)
	before, _, ok := strings.Cut(rest, "\n"+sep)
	if !ok {
		return nil
	}
	yamlBlock := before

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(yamlBlock), &raw); err != nil {
		return nil
	}
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	return &apiextensionsv1.JSON{Raw: jsonBytes}
}

func (r *PromptReconciler) patchStatus(ctx context.Context, original, updated *genkitv1alpha1.Prompt) (ctrl.Result, error) {
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *PromptReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&genkitv1alpha1.Prompt{}).
		Named("prompt").
		Complete(r)
}
