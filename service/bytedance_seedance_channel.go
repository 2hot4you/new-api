package service

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

// ValidateByteDanceSeedanceBaseURL is shared by configuration and every
// outbound reseller boundary. Errors deliberately never echo the input.
func ValidateByteDanceSeedanceBaseURL(raw string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(base, "#") || seedanceURLPort(u) <= 0 || seedanceURLPort(u) > 65535 {
		return "", errors.New("ByteDance Seedance requires an absolute HTTP(S) Base URL without credentials, query or fragment")
	}
	if self, e := url.Parse(strings.TrimSpace(system_setting.ServerAddress)); e == nil && self.Hostname() != "" && isDirectSeedanceSelfURL(u, self) {
		return "", errors.New("ByteDance Seedance Base URL cannot point to this instance")
	}
	return base, nil
}

func isDirectSeedanceSelfURL(upstream, self *url.URL) bool {
	upstreamHost := strings.TrimSuffix(strings.ToLower(upstream.Hostname()), ".")
	selfHost := strings.TrimSuffix(strings.ToLower(self.Hostname()), ".")
	if upstreamHost == selfHost {
		if upstream.Port() == "" && self.Port() == "" {
			return true
		}
		return seedanceURLPort(upstream) == seedanceURLPort(self)
	}
	loopback := func(host string) bool {
		ip := net.ParseIP(host)
		return host == "localhost" || (ip != nil && ip.IsLoopback())
	}
	return loopback(upstreamHost) && loopback(selfHost) && seedanceURLPort(upstream) == seedanceURLPort(self)
}

func seedanceURLPort(u *url.URL) int {
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil {
			return -1
		}
		return n
	}
	if strings.HasSuffix(u.Host, ":") {
		return -1
	}
	if u.Scheme == "https" {
		return 443
	}
	return 80
}
