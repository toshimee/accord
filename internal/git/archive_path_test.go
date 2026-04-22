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

package git

import "testing"

func TestArchiveRelativePath(t *testing.T) {
	got, err := ArchiveRelativePath("inventory/applications/default/test-app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := "inventory/archive/applications/default/test-app.yaml"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestArchiveRelativePath_invalid(t *testing.T) {
	_, err := ArchiveRelativePath("other/foo.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResourceNameFromInventoryPath(t *testing.T) {
	if got := resourceNameFromInventoryPath("inventory/applications/ns/my-app.yaml"); got != "my-app" {
		t.Fatalf("got %q", got)
	}
}
