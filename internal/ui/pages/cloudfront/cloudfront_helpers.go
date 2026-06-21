package cloudfront

import (
	"strings"
)

func parseInvalidationPaths(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if !strings.HasPrefix(field, "/") {
			field = "/" + field
		}
		paths = append(paths, field)
	}
	if len(paths) == 0 {
		return []string{"/*"}
	}
	return paths
}

func buildInvalidationCommand(profileName, distributionID string, paths []string) string {
	parts := []string{"aws", "cloudfront", "create-invalidation", "--distribution-id", shellQuote(distributionID), "--paths"}
	for _, path := range paths {
		parts = append(parts, shellQuote(path))
	}
	if profileName = strings.TrimSpace(profileName); profileName != "" {
		parts = append(parts, "--profile", shellQuote(profileName))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	value = strings.ReplaceAll(value, `'`, `"'"'`)
	return "'" + value + "'"
}

func firstAlias(aliases []string) string {
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			return alias
		}
	}
	return ""
}
