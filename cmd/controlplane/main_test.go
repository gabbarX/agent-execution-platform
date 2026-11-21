package main

import "testing"

func TestParseWeights(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    map[string]int
		wantErr bool
	}{
		{"valid", "a=2,b=1", map[string]int{"a": 2, "b": 1}, false},
		{"empty", "", map[string]int{}, false},
		{"spaces", " a=2 , b=1 ", map[string]int{"a": 2, "b": 1}, false},
		{"missing value", "a=", nil, true},
		{"not a number", "a=x", nil, true},
		{"negative", "a=-1", nil, true},
		{"no equals", "a", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseWeights(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parseWeights(%q) = %v, want error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWeights(%q): %v", c.in, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("got[%q] = %d, want %d", k, got[k], v)
				}
			}
		})
	}
}
