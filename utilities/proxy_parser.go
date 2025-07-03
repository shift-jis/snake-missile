package utilities

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseProxyURL(proxyString string) (*url.URL, error) {
	if proxyString == "" || len(strings.TrimSpace(proxyString)) == 0 {
		return nil, nil
	}

	proxyParts := strings.Split(proxyString, "@")
	var addressPart string
	var authPart string

	switch len(proxyParts) {
	case 1: // Only address, e.g., "host:port" or "scheme://host:port"
		addressPart = proxyParts[0]
	case 2: // "user:pass@host:port" or "user:pass@scheme://host:port"
		addressPart = proxyParts[1]
		authPart = proxyParts[0]
	default: // Invalid format
		return nil, fmt.Errorf("invalid proxy string format: too many '@' symbols in '%s'", proxyString)
	}

	if addressPart == "" || len(strings.TrimSpace(addressPart)) == 0 {
		return nil, fmt.Errorf("proxy address cannot be empty")
	}

	parseProxyURL, err := url.Parse("http://" + addressPart)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy address '%s': %w", addressPart, err)
	}

	if authPart != "" && len(strings.TrimSpace(authPart)) > 0 {
		credentials := strings.Split(authPart, ":")
		if len(credentials) == 2 {
			parseProxyURL.User = url.UserPassword(credentials[0], credentials[1])
		} else {
			return nil, fmt.Errorf("invalid proxy credentials format in '%s': expected 'user:pass'", authPart)
		}
	}

	return parseProxyURL, nil
}
