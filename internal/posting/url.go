package posting

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return "", errors.New("Enter a valid URL")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("Enter a valid URL")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)

	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" || parsed.Opaque != "" {
		return "", errors.New("Enter a valid URL")
	}

	if parsed.User != nil {
		return "", errors.New("URLs must not contain a username or password.")
	}

	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", errors.New("Enter a URL with valid query parameters")
	}

	for key := range query {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "utm_") ||
			lowerKey == "gclid" || lowerKey == "fbclid" {
			delete(query, key)
		}
	}

	parsed.Host = strings.ToLower(parsed.Host)

	if parsed.Path == "" {
		parsed.Path = "/"
		parsed.RawPath = ""
	}

	parsed.RawQuery = query.Encode()
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""

	return parsed.String(), nil
}
