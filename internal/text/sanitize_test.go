package text

import "testing"

func TestSanitize(t *testing.T) {
	esc := string(rune(0x1b))
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"  trim me  ", "trim me"},
		{esc + "[31mred" + esc + "[0m", "[31mred[0m"}, // ESC stripped, rest kept
		{"a\x00b\x07c", "abc"},                        // C0 controls
		{"x​z", "xz"},                                 // zero-width (Cf)
		{"tab\there", "tabhere"},                      // tab (0x09) is a C0 control -> stripped
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
