package common

import (
	"testing"

	cfg "github.com/ibp-network/ibp-geodns-libs/config"
)

func TestNormalizeCheckType(t *testing.T) {
	tests := map[string]string{
		"site":     "site",
		"SITE":     "site",
		"1":        "site",
		"domain":   "domain",
		"2":        "domain",
		"Endpoint": "endpoint",
		"3":        "endpoint",
		"custom":   "custom",
	}

	for input, want := range tests {
		if got := NormalizeCheckType(input); got != want {
			t.Fatalf("NormalizeCheckType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExpandCheckTypeValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "site", input: "site", want: []string{"site", "1"}},
		{name: "numeric domain", input: "2", want: []string{"domain", "2"}},
		{name: "endpoint", input: "endpoint", want: []string{"endpoint", "3"}},
		{name: "custom", input: "custom", want: []string{"custom"}},
	}

	for _, test := range tests {
		got := ExpandCheckTypeValues(test.input)
		if len(got) != len(test.want) {
			t.Fatalf("%s: len = %d, want %d", test.name, len(got), len(test.want))
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Fatalf("%s: value[%d] = %q, want %q", test.name, i, got[i], test.want[i])
			}
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	input := `Alice/Bob: Validator?`
	want := "Alice_Bob__Validator_"
	if got := SanitizeFilename(input); got != want {
		t.Fatalf("SanitizeFilename(%q) = %q, want %q", input, got, want)
	}
}

func TestMapServiceNameByEventPrefersExactEndpointPath(t *testing.T) {
	services := map[string]cfg.Service{
		"Polkadot": {
			Providers: map[string]cfg.ServiceProvider{
				"ibp": {RpcUrls: []string{"wss://rpc.ibp.network/polkadot"}},
			},
		},
		"Kusama": {
			Providers: map[string]cfg.ServiceProvider{
				"ibp": {RpcUrls: []string{"wss://rpc.ibp.network/kusama"}},
			},
		},
	}

	if got := MapServiceNameByEvent(services, "rpc.ibp.network", "wss://rpc.ibp.network/polkadot"); got != "Polkadot" {
		t.Fatalf("MapServiceNameByEvent exact endpoint = %q, want %q", got, "Polkadot")
	}
	if got := MapServiceNameByEvent(services, "rpc.ibp.network", "wss://rpc.ibp.network/kusama"); got != "Kusama" {
		t.Fatalf("MapServiceNameByEvent exact endpoint = %q, want %q", got, "Kusama")
	}
	if got := MapServiceNameByEvent(services, "rpc.ibp.network", ""); got != "" {
		t.Fatalf("MapServiceNameByEvent ambiguous domain = %q, want empty", got)
	}
}

func TestEventMatchesServiceUsesUniqueMapping(t *testing.T) {
	services := map[string]cfg.Service{
		"BridgeHub": {
			Providers: map[string]cfg.ServiceProvider{
				"dotters": {RpcUrls: []string{"wss://bridge-hub-polkadot.dotters.network"}},
			},
		},
	}

	if !EventMatchesService(services, "BridgeHub", "bridge-hub-polkadot.dotters.network", "") {
		t.Fatal("expected exact domain label to match BridgeHub")
	}
	if EventMatchesService(services, "BridgeHub", "people-polkadot.dotters.network", "") {
		t.Fatal("unexpected match for unrelated domain label")
	}
}
