package main

import "testing"

func TestIsValidUrl(t *testing.T) {
	want := []struct {
		input  string
		result string
		ok     bool
	}{
		{"https://example.com", "https://example.com", true},
		{"https://example.com/", "https://example.com/", true},
		{"https://example.com///", "https://example.com///", true},
		{"https://example.com/test/test/", "https://example.com/test/test/", true},
		{"https://example.com/test", "https://example.com/test", true},
		{"example.com", "https://example.com", true},
		{".com", "", false},
		{"https", "", false},
		{"", "", false},
		{":", "", false},
		{"file://test", "", false},
		{"http://test", "http://test", true},
		{"://", "", false},
		{"http://test test", "", false},
		{"example.com/test/test/test", "https://example.com/test/test/test", true},
		{"example.com/", "https://example.com", true},
		{" ", "", false},
	}

	for _, w := range want {
		got, gotok := isValidURL(w.input)
		if got != w.result || gotok != w.ok {
			t.Errorf("\n\x1b[31mgot unexpected results:\ninput: %s\n want: %s %t\n  got: %s %t\x1b[0m\n",
				w.input, w.result, w.ok, got, gotok)
		}
	}
}
