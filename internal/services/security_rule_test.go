package services

import "testing"

func TestNormalizeAllPortsProtocol(t *testing.T) {
	cases := []struct {
		protocol string
		min      int
		max      int
		want     string
	}{
		{protocol: "6", min: 1, max: 65535, want: "all"},
		{protocol: "17", min: 1, max: 65535, want: "all"},
		{protocol: "6", min: 0, max: 65535, want: "all"},
		{protocol: "6", min: 22, max: 22, want: "6"},
		{protocol: "6", min: 1, max: 65534, want: "6"},
		{protocol: "1", min: 1, max: 65535, want: "1"},
		{protocol: "all", min: 1, max: 65535, want: "all"},
	}
	for _, tc := range cases {
		if got := normalizeAllPortsProtocol(tc.protocol, tc.min, tc.max); got != tc.want {
			t.Fatalf("protocol=%s ports=%d-%d: got %q want %q", tc.protocol, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestOpenSecurityListRulesUseAllProtocol(t *testing.T) {
	ingress := openIngressRules(false)
	if len(ingress) != 1 {
		t.Fatalf("expected 1 ipv4 ingress rule, got %d", len(ingress))
	}
	if ingress[0].Protocol == nil || *ingress[0].Protocol != "all" {
		t.Fatalf("ingress must use all protocol, got %+v", ingress[0].Protocol)
	}
	if ingress[0].Source == nil || *ingress[0].Source != "0.0.0.0/0" {
		t.Fatalf("unexpected ingress source: %+v", ingress[0].Source)
	}

	ingressV6 := openIngressRules(true)
	if len(ingressV6) != 2 || ingressV6[1].Source == nil || *ingressV6[1].Source != "::/0" {
		t.Fatalf("expected ipv6 ingress rule, got %+v", ingressV6)
	}

	egress := openEgressRules(true)
	if len(egress) != 2 {
		t.Fatalf("expected 2 egress rules, got %d", len(egress))
	}
	for _, rule := range egress {
		if rule.Protocol == nil || *rule.Protocol != "all" {
			t.Fatalf("egress must use all protocol, got %+v", rule.Protocol)
		}
	}
}
