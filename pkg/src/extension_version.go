package src

import (
	"regexp"
	"strings"
)

var reCreateExtVersion = regexp.MustCompile(`(?i)\bVERSION\s+('[^']*'|"[^"]*"|[^\s;]+)`)

// ExtensionVersionFromDefSQL returns the WITH VERSION value from a CREATE EXTENSION deparsed string, or "".
func ExtensionVersionFromDefSQL(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return ""
	}
	sub := reCreateExtVersion.FindStringSubmatch(sql)
	if len(sub) < 2 {
		return ""
	}
	v := strings.TrimSpace(sub[1])
	v = strings.Trim(v, `'"`)
	return v
}
