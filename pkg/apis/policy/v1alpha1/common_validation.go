/*
Copyright 2020 Google LLC.

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
	"fmt"
	"net/http"

	"github.com/google/knative-gcp/pkg/apis/policy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"
)

var subjectRefErr = &apis.FieldError{
	Paths:   []string{"apiVersion", "kind", "name"},
	Message: `"apiVersion", "kind" and "name" must be specified altogether`,
}

var validHTTPMethods = sets.NewString(
	http.MethodConnect,
	http.MethodDelete,
	http.MethodGet,
	http.MethodHead,
	http.MethodOptions,
	http.MethodPatch,
	http.MethodPost,
	http.MethodPut,
	http.MethodTrace,
)

// Validate validates a StringMatch.
func (m *StringMatch) Validate(ctx context.Context) *apis.FieldError {
	matchesSpecified := make([]string, 0)
	if m.Exact != "" {
		matchesSpecified = append(matchesSpecified, "exact")
	}
	if m.Prefix != "" {
		matchesSpecified = append(matchesSpecified, "prefix")
	}
	if m.Suffix != "" {
		matchesSpecified = append(matchesSpecified, "suffix")
	}
	if m.Presence {
		matchesSpecified = append(matchesSpecified, "presence")
	}
	if len(matchesSpecified) > 1 {
		return apis.ErrMultipleOneOf(matchesSpecified...)
	}
	if len(matchesSpecified) == 0 {
		return apis.ErrMissingOneOf(
			"exact",
			"prefix",
			"suffix",
			"presence",
		)
	}
	return nil
}

// Validate validates a KeyValuesMatch.
func (kvm *KeyValuesMatch) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if kvm.Key == "" {
		errs = errs.Also(apis.ErrMissingField("key"))
	}
	if kvm.Values == nil || len(kvm.Values) == 0 {
		errs = errs.Also(apis.ErrMissingField("values"))
	}
	if err := ValidateStringMatches(ctx, kvm.Values, "values"); err != nil {
		errs = errs.Also(err)
	}
	return errs
}

// Validate validates a JWTSpec.
func (j *JWTSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if j.Issuer == "" {
		errs = errs.Also(apis.ErrMissingField("issuer"))
	}
	if j.Jwks != "" && j.JwksURI != "" {
		errs = errs.Also(apis.ErrMultipleOneOf("jwks", "jwksUri"))
	}
	if j.Jwks == "" && j.JwksURI == "" {
		errs = errs.Also(apis.ErrMissingOneOf("jwks", "jwksUri"))
	}
	if j.FromHeaders == nil || len(j.FromHeaders) == 0 {
		errs = errs.Also(apis.ErrMissingField("fromHeaders"))
	}
	return errs
}

// Validate validates a JWTRule.
func (j *JWTRule) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	for i, c := range j.Claims {
		if err := c.Validate(ctx); err != nil {
			errs = errs.Also(err.ViaFieldIndex("claims", i))
		}
	}
	return errs
}

// Validate validates a RequestOperation.
func (op *RequestOperation) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if err := ValidateStringMatches(ctx, op.Hosts, "hosts"); err != nil {
		errs = errs.Also(err)
	}
	if err := ValidateStringMatches(ctx, op.Paths, "paths"); err != nil {
		errs = errs.Also(err)
	}
	for j, m := range op.Methods {
		if !validHTTPMethods.Has(m) {
			errs = errs.Also(apis.ErrInvalidArrayValue(m, "methods", j))
		}
	}
	return errs
}

// ValidateStringMatches a slice of StringMatch.
func ValidateStringMatches(ctx context.Context, matches []StringMatch, field string) *apis.FieldError {
	if matches == nil {
		return nil
	}
	var errs *apis.FieldError
	for i, m := range matches {
		if err := m.Validate(ctx); err != nil {
			errs = errs.Also(err.ViaFieldIndex(field, i))
		}
	}
	return errs
}

// Validate validates a PolicyBindingSpec.
func (pbs *PolicyBindingSpec) Validate(ctx context.Context, parentNamespace string) *apis.FieldError {
	var errs *apis.FieldError
	if pbs.Subject.Namespace != parentNamespace {
		errs = errs.Also(apis.ErrInvalidValue(pbs.Subject.Namespace, "namespace").ViaField("subject"))
	}
	// Specify subject either by an object reference or by label selector.
	// Cannot be both.
	if !allEmpty(pbs.Subject.APIVersion, pbs.Subject.Kind, pbs.Subject.Name) &&
		!allPresent(pbs.Subject.APIVersion, pbs.Subject.Kind, pbs.Subject.Name) {
		errs = errs.Also(subjectRefErr.ViaField("subject"))
	}
	if pbs.Subject.Name == "" && pbs.Subject.Selector == nil {
		errs = errs.Also(apis.ErrMissingOneOf("name", "selector").ViaField("subject"))
	}
	if pbs.Subject.Name != "" && pbs.Subject.Selector != nil {
		errs = errs.Also(apis.ErrMultipleOneOf("name", "selector").ViaField("subject"))
	}

	if pbs.Policy.APIVersion != "" || pbs.Policy.Kind != "" {
		errs = errs.Also(apis.ErrDisallowedFields("apiVersion", "kind").ViaField("policy"))
	}
	if pbs.Policy.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name").ViaField("policy"))
	}
	if pbs.Policy.Namespace != parentNamespace {
		errs = errs.Also(apis.ErrInvalidValue(pbs.Policy.Namespace, "namespace").ViaField("policy"))
	}
	return errs
}

func allEmpty(ss ...string) bool {
	for _, s := range ss {
		if s != "" {
			return false
		}
	}
	return true
}

func allPresent(ss ...string) bool {
	for _, s := range ss {
		if s == "" {
			return false
		}
	}
	return true
}

// CheckImmutableFields checks if any immutable fields are changed in a PolicyBindingSpec.
// Make PolicyBindingSpec immutable because otherwise the following case cannot be handled properly:
// A policy binding initially binds a policy to subject A but later gets changed to subject B.
// The controller will not be aware of the previous value (subject A) and thus cannot properly unbind
// the policy from subject A.
func (pbs *PolicyBindingSpec) CheckImmutableFields(ctx context.Context, original *PolicyBindingSpec) *apis.FieldError {
	if original == nil {
		return nil
	}
	if diff, err := kmp.ShortDiff(original, pbs); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff PolicyBindingSpec",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}

// CheckImmutableBindingObjectMeta checks the immutable fields in policy binding ObjectMeta.
func CheckImmutableBindingObjectMeta(ctx context.Context, current, original *metav1.ObjectMeta) *apis.FieldError {
	if original == nil {
		return nil
	}
	var currBindingClass, originalBindingClass string
	if current.Annotations != nil {
		currBindingClass = current.Annotations[policy.PolicyBindingClassAnnotationKey]
	}
	if original.Annotations != nil {
		originalBindingClass = original.Annotations[policy.PolicyBindingClassAnnotationKey]
	}
	if currBindingClass != originalBindingClass {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"annotations", policy.PolicyBindingClassAnnotationKey},
			Details: fmt.Sprintf("-: %q\n+: %q", originalBindingClass, currBindingClass),
		}
	}
	return nil
}
