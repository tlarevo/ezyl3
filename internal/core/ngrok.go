package core

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var ngrokDomainPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.ngrok-free\.(dev|app)$`)

func NormalizeNgrokDomain(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", fmt.Errorf("ngrok domain is required")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		value = parsed.Host
	}
	value = strings.TrimSuffix(value, "/")
	value = strings.ToLower(value)
	if !ngrokDomainPattern.MatchString(value) {
		return "", fmt.Errorf("invalid ngrok free domain: %s", input)
	}
	return value, nil
}
