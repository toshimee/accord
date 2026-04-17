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

import (
	"fmt"
	"strings"
	"text/template"
)

// ArgoPluralKind returns the Git export directory segment for Argo kinds (docs/git-policy.md).
func ArgoPluralKind(kind string) string {
	switch kind {
	case "Application":
		return "applications"
	case "ApplicationSet":
		return "applicationsets"
	default:
		return strings.ToLower(kind) + "s"
	}
}

type exportPathData struct {
	PluralKind string
	Namespace  string
	Name       string
}

// RenderExportPath executes EXPORT_PATH_TEMPLATE with plural kind, namespace, and name.
func RenderExportPath(tmplStr, kind, namespace, name string) (string, error) {
	tmpl, err := template.New("exportPath").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse export path template: %w", err)
	}
	var buf strings.Builder
	data := exportPathData{
		PluralKind: ArgoPluralKind(kind),
		Namespace:  namespace,
		Name:       name,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute export path template: %w", err)
	}
	return buf.String(), nil
}
