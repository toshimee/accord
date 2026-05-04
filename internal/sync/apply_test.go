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

package sync

import (
	"context"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const goodAppYAML = `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  project: default
  destination:
    namespace: default
    server: https://kubernetes.default.svc
  source:
    repoURL: https://example.com/repo.git
    path: .
`

const goodAppSetYAML = `apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: addons
  namespace: argocd
spec:
  generators: []
`

// TestApplyInventoryYAML_pathManifestIdentity_table verifies P0 1.4: every
// mismatch axis between the inventory file path and the manifest body MUST
// surface a *PathManifestMismatchError and the cluster MUST not be touched.
func TestApplyInventoryYAML_pathManifestIdentity_table(t *testing.T) {
	cases := []struct {
		name         string
		parsed       ParsedInventoryPath
		yaml         string
		wantField    string // empty == expect success
		wantPath     string
		wantManifest string
	}{
		{
			name:   "matching Application succeeds",
			parsed: ParsedInventoryPath{PluralKind: "applications", Namespace: "argocd", Name: "my-app"},
			yaml:   goodAppYAML,
		},
		{
			name:   "matching ApplicationSet succeeds",
			parsed: ParsedInventoryPath{PluralKind: "applicationsets", Namespace: "argocd", Name: "addons"},
			yaml:   goodAppSetYAML,
		},
		{
			name: "wrong apiVersion is rejected",
			parsed: ParsedInventoryPath{
				PluralKind: "applications", Namespace: "argocd", Name: "my-app",
			},
			yaml: strings.Replace(goodAppYAML,
				"apiVersion: argoproj.io/v1alpha1",
				"apiVersion: rbac.authorization.k8s.io/v1", 1),
			wantField:    "apiVersion",
			wantPath:     argoExpectedAPIVersion,
			wantManifest: "rbac.authorization.k8s.io/v1",
		},
		{
			name: "wrong kind for plural is rejected",
			parsed: ParsedInventoryPath{
				PluralKind: "applications", Namespace: "argocd", Name: "my-app",
			},
			// path says applications -> Kind=Application, but manifest has Kind=ApplicationSet
			yaml: strings.Replace(goodAppYAML,
				"kind: Application\n",
				"kind: ApplicationSet\n", 1),
			wantField:    "kind",
			wantPath:     "Application",
			wantManifest: "ApplicationSet",
		},
		{
			name: "wrong namespace is rejected",
			parsed: ParsedInventoryPath{
				PluralKind: "applications", Namespace: "argocd", Name: "my-app",
			},
			yaml: strings.Replace(goodAppYAML,
				"namespace: argocd",
				"namespace: kube-system", 1),
			wantField:    "namespace",
			wantPath:     "argocd",
			wantManifest: "kube-system",
		},
		{
			name: "wrong name is rejected",
			parsed: ParsedInventoryPath{
				PluralKind: "applications", Namespace: "argocd", Name: "my-app",
			},
			yaml: strings.Replace(goodAppYAML,
				"name: my-app\n",
				"name: other-app\n", 1),
			wantField:    "name",
			wantPath:     "my-app",
			wantManifest: "other-app",
		},
		{
			name: "missing namespace in manifest is rejected",
			parsed: ParsedInventoryPath{
				PluralKind: "applications", Namespace: "argocd", Name: "my-app",
			},
			yaml: strings.Replace(goodAppYAML,
				"  namespace: argocd\n",
				"", 1),
			wantField:    "namespace",
			wantPath:     "argocd",
			wantManifest: "",
		},
		{
			name: "ClusterRoleBinding shaped manifest at applications path is rejected on apiVersion",
			parsed: ParsedInventoryPath{
				PluralKind: "applications", Namespace: "argocd", Name: "my-app",
			},
			yaml: `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-app
  namespace: argocd
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects: []
`,
			wantField:    "apiVersion",
			wantPath:     argoExpectedAPIVersion,
			wantManifest: "rbac.authorization.k8s.io/v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := fake.NewClientBuilder().WithScheme(argoScheme(t)).Build()
			err := ApplyInventoryYAML(context.Background(), k, tc.parsed, []byte(tc.yaml))

			if tc.wantField == "" {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected mismatch on %q, got nil", tc.wantField)
			}
			if !IsPathManifestMismatch(err) {
				t.Fatalf("expected *PathManifestMismatchError, got %T: %v", err, err)
			}
			var got *PathManifestMismatchError
			// errors.As is wrapped inside IsPathManifestMismatch; re-extract here for field assertion.
			if !asMismatch(err, &got) {
				t.Fatalf("could not extract *PathManifestMismatchError: %v", err)
			}
			if got.Field != tc.wantField {
				t.Errorf("Field: got %q, want %q", got.Field, tc.wantField)
			}
			if got.PathExpect != tc.wantPath {
				t.Errorf("PathExpect: got %q, want %q", got.PathExpect, tc.wantPath)
			}
			if got.ManifestSeen != tc.wantManifest {
				t.Errorf("ManifestSeen: got %q, want %q", got.ManifestSeen, tc.wantManifest)
			}
		})
	}
}

// asMismatch is a tiny errors.As wrapper for the table test.
func asMismatch(err error, target **PathManifestMismatchError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*PathManifestMismatchError); ok {
		*target = e
		return true
	}
	return false
}
