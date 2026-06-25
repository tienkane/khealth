package check

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDurationUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"500ms", 500 * time.Millisecond},
		{"2s", 2 * time.Second},
		{"1m30s", 90 * time.Second},
		{"3", 3 * time.Second}, // bare number => seconds
		{"0.5", 500 * time.Millisecond},
	}
	for _, c := range cases {
		var d Duration
		if err := yaml.Unmarshal([]byte(c.in), &d); err != nil {
			t.Fatalf("%q: unexpected error: %v", c.in, err)
		}
		if d.D() != c.want {
			t.Errorf("%q => %v, want %v", c.in, d.D(), c.want)
		}
	}
}

func TestDurationUnmarshalInvalid(t *testing.T) {
	var d Duration
	if err := yaml.Unmarshal([]byte("nope"), &d); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
