package graph

import "testing"

func TestUnmarshalDecimal_AcceptsFormattedStrings(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		{"20000", "20000"},
		{"20,000", "20000"},
		{"MMK 20,000", "20000"},
		{"MMK -20,000", "-20000"},
		{"  ks 1,234.50  ", "1234.5"},
	}
	for _, tc := range cases {
		d, err := UnmarshalDecimal(tc.in)
		if err != nil {
			t.Fatalf("UnmarshalDecimal(%q) error: %v", tc.in, err)
		}
		if d.String() != tc.expected {
			t.Fatalf("UnmarshalDecimal(%q) expected %s, got %s", tc.in, tc.expected, d.String())
		}
	}
}

