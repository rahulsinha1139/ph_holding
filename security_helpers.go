package main

import (
	"regexp"
	"strings"
)

// escapeLikeWildcards escapes SQL LIKE wildcards (%, _) for safe LIKE queries
// SECURITY: Prevents LIKE injection attacks by escaping wildcards with backslash
// Usage: db.Where("name LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(userInput)+"%")
func escapeLikeWildcards(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslash first
	s = strings.ReplaceAll(s, "%", "\\%")    // Escape percent
	s = strings.ReplaceAll(s, "_", "\\_")    // Escape underscore
	return s
}

// isValidSQLIdentifier validates that a string is a safe SQL identifier
// SECURITY: Prevents SQL injection in DDL statements (DROP INDEX, ALTER TABLE)
// Only allows alphanumeric characters and underscores, starting with letter or underscore
func isValidSQLIdentifier(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return matched
}
