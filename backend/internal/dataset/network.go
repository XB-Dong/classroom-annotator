package dataset

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Policy struct{ PrivateOrigins, MediaOrigins []string }

func Origin(u *url.URL) string { return strings.ToLower(u.Scheme + "://" + u.Host) }
func contains(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
func (p *Policy) Validate(u *url.URL, media bool) error {
	if u == nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Fragment != "" {
		return ErrUpstream
	}
	if media && len(p.MediaOrigins) > 0 && !contains(p.MediaOrigins, Origin(u)) {
		return ErrUpstream
	}
	return nil
}
func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range []string{"100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4"} {
		_, n, _ := net.ParseCIDR(block)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
func (p *Policy) Transport() *http.Transport {
	return &http.Transport{TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second, MaxIdleConns: 4, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		allow := false
		for _, o := range p.PrivateOrigins {
			u, e := url.Parse(o)
			if e != nil {
				continue
			}
			want := u.Port()
			if want == "" {
				want = "80"
				if u.Scheme == "https" {
					want = "443"
				}
			}
			if strings.EqualFold(u.Hostname(), host) && want == port {
				allow = true
			}
		}
		ips, e := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if e != nil || len(ips) == 0 {
			return nil, errors.New("destination unavailable")
		}
		for _, ip := range ips {
			if !allow && !publicIP(ip) {
				return nil, errors.New("destination rejected")
			}
		}
		var last error
		for _, ip := range ips {
			conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}}
}
