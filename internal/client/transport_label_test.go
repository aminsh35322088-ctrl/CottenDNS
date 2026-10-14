package client

import "testing"

func TestResolverTransportLabelForEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		transport resolverTransport
		endpoint  string
		want      string
	}{
		{name: "udp custom ipv4", transport: transportUDP, endpoint: "1.1.1.1:43629", want: "UDP/43629"},
		{name: "udp custom ipv6", transport: transportUDP, endpoint: "[2001:db8::1]:43629", want: "UDP/43629"},
		{name: "udp host only", transport: transportUDP, endpoint: "1.1.1.1", want: "UDP/53"},
		{name: "udp malformed port", transport: transportUDP, endpoint: "1.1.1.1:not-a-port", want: "UDP/53"},
		{name: "tcp custom", transport: transportTCP, endpoint: "9.9.9.9:5300", want: "TCP/5300"},
		{name: "tcp host only", transport: transportTCP, endpoint: "9.9.9.9", want: "TCP/53"},
		{name: "dot unchanged", transport: transportDoT, endpoint: "1.1.1.1:43629", want: "DoT"},
		{name: "doh unchanged", transport: transportDoH, endpoint: "1.1.1.1:43629", want: "DoH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.transport.labelForEndpoint(tt.endpoint); got != tt.want {
				t.Fatalf("labelForEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestClientTransportStatusLabelUsesActiveCustomPort(t *testing.T) {
	c := &Client{connections: []Connection{
		{IsValid: true, ResolverLabel: "1.1.1.1:43629"},
		{IsValid: true, ResolverLabel: "9.9.9.9:43629"},
	}}
	c.setActiveTransport(transportUDP)
	if got := c.transportStatusLabel(); got != "UDP/43629" {
		t.Fatalf("transportStatusLabel() = %q, want UDP/43629", got)
	}
}

func TestClientTransportStatusLabelReportsMixedPortsWithoutLying(t *testing.T) {
	c := &Client{connections: []Connection{
		{IsValid: true, ResolverLabel: "1.1.1.1:43629"},
		{IsValid: true, ResolverLabel: "9.9.9.9:5300"},
	}}
	c.setActiveTransport(transportUDP)
	if got := c.transportStatusLabel(); got != "UDP/mixed" {
		t.Fatalf("transportStatusLabel() = %q, want UDP/mixed", got)
	}
}
