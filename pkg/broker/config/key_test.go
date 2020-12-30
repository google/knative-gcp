/*
Copyright 2020 Google LLC

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

package config

import (
	"testing"
)

func TestBrokerKeyPersistenceString(t *testing.T) {
	testCases := map[string]struct {
		key  CellTenantKey
		want string
	}{
		"broker": {
			key: CellTenantKey{
				cellTenantType: CellTenantType_BROKER,
				namespace:      "my-namespace",
				name:           "my-name",
			},
			want: "my-namespace/my-name",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got := tc.key.PersistenceString()
			if got != tc.want {
				t.Fatalf("Unexpected perisistence string, want %q, got %q", tc.want, got)
			}
			if got == tc.key.String() {
				t.Fatalf("Key's PersistenceString() and String() are equal, they should differ (see comment in String()): %q", got)
			}
		})
	}
}

func TestCellTenantKeyFromPersistenceString(t *testing.T) {
	testCases := map[string]struct {
		s       string
		want    *CellTenantKey
		wantErr bool
	}{
		"empty": {
			s:       "",
			wantErr: true,
		},
		"too short": {
			s:       "/foo",
			wantErr: true,
		},
		"too long": {
			s:       "/foo/bar/baz/qux",
			wantErr: true,
		},
		"no leading slash": {
			s:       "foo/BROKER/bar/baz",
			wantErr: true,
		},
		"invalid namespace": {
			s:       "/BROKER/_foo/bar",
			wantErr: true,
		},
		"invalid name": {
			s:       "/BROKER/foo/_bar",
			wantErr: true,
		},
		"original broker format - no leading slash": {
			s:       "foo/bar/baz",
			wantErr: true,
		},
		"original broker format - invalid namespace": {
			s:       "/_foo/bar",
			wantErr: true,
		},
		"original broker format - invalid name": {
			s:       "/foo/_bar",
			wantErr: true,
		},
		"original broker format - valid": {
			s: "/my-ns/my-name",
			want: &CellTenantKey{
				cellTenantType: CellTenantType_BROKER,
				namespace:      "my-ns",
				name:           "my-name",
			},
		},
		"broker": {
			s: "/BROKER/my-ns/my-name",
			want: &CellTenantKey{
				cellTenantType: CellTenantType_BROKER,
				namespace:      "my-ns",
				name:           "my-name",
			},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got, err := CellTenantKeyFromPersistenceString(tc.s)
			if errIsNil := err == nil; tc.wantErr == errIsNil {
				t.Errorf("Unexpected error. Wanted %v, Got %v", tc.wantErr, err)
				return
			}
			if wantNil, gotNil := tc.want == nil, got == nil; wantNil != gotNil {
				t.Errorf("Unexpected CellTenantKey. Wanted %v, Got %v", tc.want, got)
			}
			if got != nil && *got != *tc.want {
				t.Errorf("Unexpected CellTenantKey. Wanted %v, Got %v", tc.want, got)
			}
		})
	}
}
