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

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveInventoryFileInClone(t *testing.T) {
	root := t.TempDir()
	inv := filepath.Join(root, "inventory", "applications", "default")
	if err := os.MkdirAll(inv, 0o755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(inv, "test-app.yaml")
	if err := os.WriteFile(live, []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := archiveInventoryFileInClone(root, "inventory/applications/default/test-app.yaml"); err != nil {
		t.Fatal(err)
	}
	arch := filepath.Join(root, "inventory", "archive", "applications", "default", "test-app.yaml")
	if _, err := os.Stat(arch); err != nil {
		t.Fatalf("archived file: %v", err)
	}
	if _, err := os.Stat(live); !os.IsNotExist(err) {
		t.Fatal("expected live file removed")
	}
}

func TestArchiveInventoryFileInClone_missingIsNoop(t *testing.T) {
	root := t.TempDir()
	if err := archiveInventoryFileInClone(root, "inventory/applications/default/ghost.yaml"); err != nil {
		t.Fatal(err)
	}
}
