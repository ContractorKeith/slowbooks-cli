package client

import (
	"context"
	"net"
	"net/http"
	"net/url"
)

type lookupFunc func(context.Context, string) ([]net.IPAddr, error)

func defaultLookup(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// transportFor pins HTTP connections to checked addresses, preventing DNS rebinding.
func transportFor(ctx context.Context, base *url.URL, lookup lookupFunc) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if base.Scheme == "https" {
		return transport, nil
	}
	addresses, err := lookup(ctx, base.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, usage("cannot verify HTTP host; use HTTPS")
	}
	for _, address := range addresses {
		ip := address.IP
		if !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() {
			return nil, usage("HTTPS is required for public hosts")
		}
	}
	// A proxy must not forward cleartext credentials outside the checked network.
	transport.Proxy = nil
	port := base.Port()
	if port == "" {
		port = "80"
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		var last error
		for _, checked := range addresses {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(checked.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
	return transport, nil
}
