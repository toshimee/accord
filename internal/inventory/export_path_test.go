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

package inventory

import "testing"

func TestRenderExportPath_defaultTemplate(t *testing.T) {
	tmpl := "inventory/{{.PluralKind}}/{{.Namespace}}/{{.Name}}.yaml"
	got, err := RenderExportPath(tmpl, "Application", "argocd", "my-app")
	if err != nil {
		t.Fatal(err)
	}
	want := "inventory/applications/argocd/my-app.yaml"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got2, err := RenderExportPath(tmpl, "ApplicationSet", "argocd", "cluster-addons")
	if err != nil {
		t.Fatal(err)
	}
	want2 := "inventory/applicationsets/argocd/cluster-addons.yaml"
	if got2 != want2 {
		t.Fatalf("got %q want %q", got2, want2)
	}
}
