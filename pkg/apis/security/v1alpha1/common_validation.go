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

	"knative.dev/pkg/apis"
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
	if pbs.Subject.Name == "" && pbs.Subject.Selector == nil {
		errs = errs.Also(apis.ErrMissingOneOf("name", "selector").ViaField("subject"))
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
