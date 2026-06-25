package check

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that reads from YAML as either a Go duration
// string ("500ms", "2s") or a bare number interpreted as seconds.
type Duration time.Duration

// D returns the value as a time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err == nil {
		if parsed, perr := time.ParseDuration(s); perr == nil {
			*d = Duration(parsed)
			return nil
		}
	}
	var n float64
	if err := node.Decode(&n); err == nil {
		*d = Duration(time.Duration(n * float64(time.Second)))
		return nil
	}
	return fmt.Errorf("invalid duration %q (use %q or a number of seconds)", node.Value, "500ms")
}

func (d Duration) MarshalYAML() (any, error) {
	if d == 0 {
		return nil, nil
	}
	return time.Duration(d).String(), nil
}
