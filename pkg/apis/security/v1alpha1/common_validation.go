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
	notEmpty := 0
	if m.Exact != "" {
		notEmpty++
	}
	if m.Prefix != "" {
		notEmpty++
	}
	if m.Suffix != "" {
		notEmpty++
	}
	if m.Presence {
		notEmpty++
	}
	if notEmpty > 1 {
		return apis.ErrMultipleOneOf(
			"exact",
			"prefix",
			"suffix",
			"presence",
		)
	}
	if notEmpty == 0 {
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
	if j.JwtHeader == "" {
		errs = errs.Also(apis.ErrMissingField("jwtHeader"))
	}
	if err := ValidateStringMatches(ctx, j.ExcludePaths, "excludePaths"); err != nil {
		errs = errs.Also(err)
	}
	if err := ValidateStringMatches(ctx, j.IncludePaths, "includePaths"); err != nil {
		errs = errs.Also(err)
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
