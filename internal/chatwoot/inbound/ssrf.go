package inbound

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// maxAttachmentRedirects caps the redirect hops followed for a Chatwoot
// data_url fetch. The cap is enforced per hop together with the SSRF policy,
// so a public URL cannot bounce into link-local/metadata space.
const maxAttachmentRedirects = 3

// lookupIP resolves a hostname to addresses for the SSRF policy. It is a
// variable so tests can stub resolution without network access.
var lookupIP = net.LookupIP

var (
	// metadataIPv4 are cloud metadata endpoints that are always blocked,
	// even when they match the Chatwoot host (no legitimate Chatwoot serves
	// attachments from the instance-metadata address).
	metadataIPv4 = []net.IP{
		net.ParseIP("169.254.169.254"),
		net.ParseIP("100.100.100.200"),
	}
	// linkLocalNets are always blocked regardless of the Chatwoot host.
	linkLocalNets = []net.IPNet{
		{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)},
		{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},
	}
	// carrierGradeNAT is blocked for non-Chatwoot hosts (it carries the
	// Alibaba metadata endpoint); the exact metadata IP above is blocked
	// unconditionally.
	carrierGradeNAT = net.IPNet{IP: net.ParseIP("100.64.0.0"), Mask: net.CIDRMask(10, 32)}
)

// validateAttachmentURL enforces the SSRF policy for Chatwoot data_url
// fetches: scheme http/https only, no userinfo, redirect hops revalidated by
// the caller, link-local/metadata always blocked, and private/loopback IPs
// allowed only when the resolved host equals the instance's configured
// Chatwoot url host (the self-hosted case). A rejection fails the attachment
// into a private note, never the webhook: the caller still answers 200.
func validateAttachmentURL(rawURL, chatwootURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return fmt.Errorf("attachment URL rejected: empty url")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("attachment URL rejected: unparseable url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("attachment URL rejected: scheme %q is not http or https", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("attachment URL rejected: userinfo is not allowed")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("attachment URL rejected: empty host")
	}
	ips, err := resolveAttachmentHost(host)
	if err != nil {
		return fmt.Errorf("attachment URL rejected: cannot resolve host %q", host)
	}
	for _, ip := range ips {
		if isAlwaysBlocked(ip) {
			return fmt.Errorf("attachment URL rejected: host %q resolves to blocked address", host)
		}
	}
	if chatwootHostMatches(host, chatwootURL) {
		return nil
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("attachment URL rejected: host %q resolves to non-public address", host)
		}
	}
	return nil
}

// resolveAttachmentHost returns the IPs of host without network access for
// literals, via lookupIP for names.
func resolveAttachmentHost(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ips, err := lookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	return ips, nil
}

// isAlwaysBlocked reports the addresses no attachment may come from, even
// self-hosted Chatwoot: cloud metadata endpoints and link-local ranges.
func isAlwaysBlocked(ip net.IP) bool {
	for _, meta := range metadataIPv4 {
		if ip.Equal(meta) {
			return true
		}
	}
	for _, network := range linkLocalNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// isBlockedIP reports non-public addresses: loopback, private, link-local,
// multicast, unspecified and carrier-grade NAT. Callers skip this check when
// the host matches the configured Chatwoot host.
func isBlockedIP(ip net.IP) bool {
	switch {
	case ip.IsUnspecified(),
		ip.IsLoopback(),
		ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(),
		ip.IsMulticast(),
		ip.IsPrivate():
		return true
	}
	return carrierGradeNAT.Contains(ip)
}

// pinnedAttachmentDialContext resolves once per dial, enforces the
// attachment SSRF policy against allowHost, and connects the validated IP.
// One resolution feeds both check and connect, closing the TOCTOU between
// validateAttachmentURL and the transport dial (DNS rebinding): even if the
// upfront validation saw a benign address, the dial re-resolves and refuses
// whatever the name points at now.
func pinnedAttachmentDialContext(allowHost string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSuffix(strings.ToLower(host), ".")
		ips, err := resolveAttachmentHost(trimmed)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("attachment dial rejected: cannot resolve host %q", host)
		}
		for _, ip := range ips {
			if isAlwaysBlocked(ip) {
				return nil, fmt.Errorf("attachment dial rejected: host %q resolves to blocked address", host)
			}
		}
		if !chatwootHostMatches(trimmed, allowHost) {
			for _, ip := range ips {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("attachment dial rejected: host %q resolves to non-public address", host)
				}
			}
		}
		dialer := &net.Dialer{}
		// Dial the first validated IP: the pinning uses the same
		// resolution validated above, without re-resolving.
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}

// chatwootHostMatches reports the self-hosted exception: host equals the
// configured Chatwoot URL host (case-insensitive). A different hostname is
// never trusted because its DNS can overlap a public Chatwoot address while
// also resolving to a private address. URL validation and every dial apply
// this same identity check before allowing non-public addresses.
func chatwootHostMatches(host, chatwootURL string) bool {
	trimmed := strings.TrimSpace(chatwootURL)
	if trimmed == "" {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	want := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if want == "" {
		return false
	}
	// Different textual forms of the same literal IP are the same trusted
	// identity. DNS aliases still require the configured hostname itself.
	if configuredIP, targetIP := net.ParseIP(want), net.ParseIP(host); configuredIP != nil && targetIP != nil {
		return configuredIP.Equal(targetIP)
	}
	return host == want
}
