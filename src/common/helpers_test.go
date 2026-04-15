package common

import "testing"

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

func TestEventMatchesService(t *testing.T) {
	serviceHosts := NormalizeHosts([]string{
		"wss://polkadot.dotters.network/public",
		"https://rpc.ibp.network",
	})

	tests := []struct {
		name      string
		domain    string
		endpoint  string
		wantMatch bool
	}{
		{name: "exact domain", domain: "polkadot.dotters.network", wantMatch: true},
		{name: "endpoint url host", endpoint: "wss://rpc.ibp.network/ws", wantMatch: true},
		{name: "subdomain host", endpoint: "node1.polkadot.dotters.network:9944", wantMatch: true},
		{name: "broad parent domain does not match", domain: "dotters.network", wantMatch: false},
		{name: "different host", endpoint: "https://example.com", wantMatch: false},
	}

	for _, test := range tests {
		if got := EventMatchesService(test.domain, test.endpoint, serviceHosts); got != test.wantMatch {
			t.Fatalf("%s: EventMatchesService(%q, %q) = %v, want %v", test.name, test.domain, test.endpoint, got, test.wantMatch)
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
