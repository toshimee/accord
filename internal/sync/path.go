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
	"fmt"
	"path"
	"strings"
)

// WebhookPath is the HTTP path for Git provider push webhooks (docs/phase1-sync.md).
const WebhookPath = "/api/v1/webhook"

// ParsedInventoryPath holds a validated inventory export path segment.
type ParsedInventoryPath struct {
	PluralKind string
	Namespace  string
	Name       string
}

// ParseInventoryExportPath returns (parsed, true) only for
// inventory/applications/<namespace>/<name>.yaml or inventory/applicationsets/<namespace>/<name>.yaml.
func ParseInventoryExportPath(gitPath string) (ParsedInventoryPath, bool) {
	p := path.Clean(strings.TrimSpace(gitPath))
	p = strings.TrimPrefix(p, "/")
	const prefix = "inventory/"
	if !strings.HasPrefix(p, prefix) {
		return ParsedInventoryPath{}, false
	}
	rest := strings.TrimPrefix(p, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return ParsedInventoryPath{}, false
	}
	plural, ns, file := parts[0], parts[1], parts[2]
	if plural != "applications" && plural != "applicationsets" {
		return ParsedInventoryPath{}, false
	}
	if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
		return ParsedInventoryPath{}, false
	}
	name := strings.TrimSuffix(strings.TrimSuffix(file, ".yaml"), ".yml")
	if ns == "" || name == "" || name == "." || ns == "." {
		return ParsedInventoryPath{}, false
	}
	return ParsedInventoryPath{PluralKind: plural, Namespace: ns, Name: name}, true
}

// ArgoKindFromPlural maps export directory segment to API Kind for unstructured objects.
func ArgoKindFromPlural(plural string) (string, error) {
	switch plural {
	case "applications":
		return "Application", nil
	case "applicationsets":
		return "ApplicationSet", nil
	default:
		return "", fmt.Errorf("unsupported plural kind %q", plural)
	}
}
