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
	"fmt"
	"path"
	"strings"
)

const inventoryPrefix = "inventory/"

// ArchiveRelativePath maps a live inventory file path to the same relative
// path under inventory/archive/ per docs/architecture.md §7.4, e.g.
// inventory/applications/default/test-app.yaml
// -> inventory/archive/applications/default/test-app.yaml
func ArchiveRelativePath(inventoryRel string) (string, error) {
	if !strings.HasPrefix(inventoryRel, inventoryPrefix) {
		return "", fmt.Errorf("path %q must start with %q", inventoryRel, inventoryPrefix)
	}
	rest := strings.TrimPrefix(inventoryRel, inventoryPrefix)
	return inventoryPrefix + "archive/" + rest, nil
}

// resourceNameFromInventoryPath returns the file stem (e.g. app name) of an inventory file path.
func resourceNameFromInventoryPath(inventoryRel string) string {
	base := path.Base(inventoryRel)
	return strings.TrimSuffix(base, ".yaml")
}
