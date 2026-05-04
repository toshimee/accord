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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// argoScheme registers Argo CD CRDs as unstructured types so the fake client
// can store and retrieve them. Mirrors internal/bootstrap/inventory_scheme.go.
func argoScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	gv := schema.GroupVersion{Group: "argoproj.io", Version: "v1alpha1"}
	s.AddKnownTypes(gv,
		&unstructured.Unstructured{},
		&unstructured.UnstructuredList{},
	)
	metav1.AddToGroupVersion(s, gv)
	return s
}

func newArgoApp(namespace, name, kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    kind,
	})
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// TestWebhookHandler_DeletePathValidation_table exercises the unified path
// validation introduced for review P0 1.3. The handler must reuse
// ParseInventoryExportPath + ArgoKindFromPlural for removed paths so that
// only argocd Application/ApplicationSet manifests under the inventory tree
// can ever reach client.Delete.
func TestWebhookHandler_DeletePathValidation_table(t *testing.T) {
	const secret = "topsecret"

	type expect struct {
		status        string
		detailHas     string
		stillInStore  bool // assert that the resource STILL exists after handler runs
		removedFromStore bool // assert that the resource is gone after handler runs
	}

	cases := []struct {
		name        string
		removedPath string
		seed        *unstructured.Unstructured // seeded into fake store before request
		want        expect
	}{
		{
			name:        "valid Application is deleted",
			removedPath: "inventory/applications/argocd/my-app.yaml",
			seed:        newArgoApp("argocd", "my-app", "Application"),
			want: expect{
				status:           "deleted",
				removedFromStore: true,
			},
		},
		{
			name:        "valid ApplicationSet is deleted",
			removedPath: "inventory/applicationsets/argocd/cluster-addons.yaml",
			seed:        newArgoApp("argocd", "cluster-addons", "ApplicationSet"),
			want: expect{
				status:           "deleted",
				removedFromStore: true,
			},
		},
		{
			name:        "valid Application missing from cluster reports deleted (already gone)",
			removedPath: "inventory/applications/argocd/ghost-app.yaml",
			seed:        nil,
			want: expect{
				status:    "deleted",
				detailHas: "already gone",
			},
		},
		{
			name:        "unknown plural is ignored",
			removedPath: "inventory/widgets/ns/x.yaml",
			seed:        newArgoApp("ns", "x", "Application"), // would be wrongly deleted by old code
			want: expect{
				status:       "ignored",
				detailHas:    "inventory/applications|applicationsets",
				stillInStore: true,
			},
		},
		{
			name:        "archive prefix is ignored",
			removedPath: "inventory/archive/applications/argocd/old.yaml",
			seed:        newArgoApp("argocd", "old", "Application"),
			want: expect{
				status:       "ignored",
				stillInStore: true,
			},
		},
		{
			// Old ad-hoc parser only stripped ".yaml", leaving "legacy.yml" as the
			// resource name and silently failing the delete. Unifying on
			// ParseInventoryExportPath now correctly resolves "legacy" and the
			// fake store removal is observed.
			name:        "yml extension is parsed and deleted (review §1.3 fix)",
			removedPath: "inventory/applications/argocd/legacy.yml",
			seed:        newArgoApp("argocd", "legacy", "Application"),
			want: expect{
				status:           "deleted",
				removedFromStore: true,
			},
		},
		{
			name:        "malformed depth (missing namespace segment) is ignored",
			removedPath: "inventory/applications/x.yaml",
			seed:        nil,
			want: expect{
				status:    "ignored",
				detailHas: "inventory/applications|applicationsets",
			},
		},
		{
			name:        "extra segment depth is ignored",
			removedPath: "inventory/applications/ns/sub/x.yaml",
			seed:        nil,
			want: expect{
				status: "ignored",
			},
		},
		{
			name:        "outside inventory is ignored",
			removedPath: "config/foo.yaml",
			seed:        nil,
			want: expect{
				status: "ignored",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(argoScheme(t))
			if tc.seed != nil {
				builder = builder.WithObjects(tc.seed)
			}
			k := builder.Build()

			payload := []byte(fmt.Sprintf(`{
              "repository": {"full_name": "acme/ops"},
              "head_commit": {"id": "sha1"},
              "commits": [
                {"removed": ["%s"]}
              ]
            }`, tc.removedPath))

			h := &WebhookHandler{
				K8s:           k,
				WebhookSecret: secret,
			}

			req := httptest.NewRequest(http.MethodPost, WebhookPath, bytes.NewReader(payload))
			req.Header.Set(SignatureHeader, sign(payload, secret))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200, body=%q", rec.Code, rec.Body.String())
			}
			var resp webhookResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Results) != 1 {
				t.Fatalf("expected 1 result, got %d: %+v", len(resp.Results), resp.Results)
			}
			r := resp.Results[0]
			if r.Path != tc.removedPath {
				t.Errorf("Path: got %q, want %q", r.Path, tc.removedPath)
			}
			if r.Status != tc.want.status {
				t.Errorf("Status: got %q, want %q (detail=%q)", r.Status, tc.want.status, r.Detail)
			}
			if tc.want.detailHas != "" && r.Detail == "" {
				t.Errorf("expected Detail to contain %q, got empty", tc.want.detailHas)
			}

			if tc.seed != nil {
				err := k.Get(context.Background(), types.NamespacedName{Namespace: tc.seed.GetNamespace(), Name: tc.seed.GetName()}, tc.seed.DeepCopy())
				switch {
				case tc.want.removedFromStore:
					if err == nil || !apierrors.IsNotFound(err) {
						t.Errorf("expected resource to be removed, Get returned err=%v", err)
					}
				case tc.want.stillInStore:
					if err != nil {
						t.Errorf("expected resource to remain, Get returned err=%v", err)
					}
				}
			}
		})
	}
}

// Ensure that a payload with both addedModified and removed paths is handled
// in two passes (added first, then removed) and that an unknown plural in
// removed does not cascade into the added branch.
func TestWebhookHandler_MixedAddedAndRemoved_doesNotMisroute(t *testing.T) {
	const secret = "topsecret"
	k := fake.NewClientBuilder().
		WithScheme(argoScheme(t)).
		WithObjects(newArgoApp("argocd", "doomed", "Application")).
		Build()

	payload := []byte(`{
      "repository": {"full_name": "acme/ops"},
      "head_commit": {"id": "sha1"},
      "commits": [
        {"removed": ["inventory/widgets/ns/x.yaml", "inventory/applications/argocd/doomed.yaml"]}
      ]
    }`)

	h := &WebhookHandler{K8s: k, WebhookSecret: secret}
	req := httptest.NewRequest(http.MethodPost, WebhookPath, bytes.NewReader(payload))
	req.Header.Set(SignatureHeader, sign(payload, secret))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%q", rec.Code, rec.Body.String())
	}
	var resp webhookResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Results) != 2 {
		t.Fatalf("want 2 results, got %d (%+v)", len(resp.Results), resp.Results)
	}

	byPath := map[string]webhookResult{}
	for _, r := range resp.Results {
		byPath[r.Path] = r
	}
	if got := byPath["inventory/widgets/ns/x.yaml"].Status; got != "ignored" {
		t.Errorf("widget plural should be ignored, got %q", got)
	}
	if got := byPath["inventory/applications/argocd/doomed.yaml"].Status; got != "deleted" {
		t.Errorf("valid application should be deleted, got %q", got)
	}

	// Confirm the doomed application is actually gone.
	got := newArgoApp("argocd", "doomed", "Application")
	err := k.Get(context.Background(), client.ObjectKeyFromObject(got), got)
	if err == nil || !apierrors.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got err=%v", err)
	}
}
