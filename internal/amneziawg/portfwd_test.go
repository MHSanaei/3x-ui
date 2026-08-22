package amneziawg

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestForwardedPortsInclude(t *testing.T) {
	cases := []struct {
		spec string
		port int
		want bool
	}{
		{"80,443", 80, true},
		{"80,443", 443, true},
		{"80,443", 8080, false},
		{"8000-8100", 8050, true},
		{"8000-8100", 7999, false},
		{"8000-8100", 8101, false},
		{"", 80, false},
		{"not-a-port", 80, false},
	}
	for _, c := range cases {
		if got := ForwardedPortsInclude(c.spec, c.port); got != c.want {
			t.Errorf("ForwardedPortsInclude(%q, %d) = %v, want %v", c.spec, c.port, got, c.want)
		}
	}
}

func TestExpandForwardedPorts(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want []int
	}{
		{"empty", "", nil},
		{"malformed", "not-a-port", nil},
		{"single ports", "443,80", []int{80, 443}},
		{"a range", "8000-8003", []int{8000, 8001, 8002, 8003}},
		{
			"overlapping-but-distinct ranges dedupe and merge",
			"80-90,85-95",
			[]int{80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95},
		},
		{"mixed single ports and a range, unsorted input", "443,80-82,80", []int{80, 81, 82, 443}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpandForwardedPorts(c.spec)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ExpandForwardedPorts(%q) = %v, want %v", c.spec, got, c.want)
			}
		})
	}
}

func TestExpandForwardedPortsCapsAtMaxForwardedPorts(t *testing.T) {
	got := ExpandForwardedPorts("1-200")
	if len(got) != MaxForwardedPorts {
		t.Fatalf("len(ExpandForwardedPorts(\"1-200\")) = %d, want %d", len(got), MaxForwardedPorts)
	}
	for i, port := range got {
		if want := i + 1; port != want {
			t.Fatalf("ExpandForwardedPorts(\"1-200\")[%d] = %d, want %d (expansion must stop at the cap, not truncate after expanding fully)", i, port, want)
		}
	}
}

func TestExpandForwardedPortsCapAppliesAcrossMultipleSpecs(t *testing.T) {
	// A spec whose total span far exceeds the cap, split across many
	// individually-small tokens -- proves the cap is enforced cumulatively
	// across specs, not reset (or bypassed) per spec.
	tokens := make([]string, 150)
	for i := range tokens {
		tokens[i] = strconv.Itoa(10000 + i)
	}
	spec := strings.Join(tokens, ",")
	got := ExpandForwardedPorts(spec)
	if len(got) != MaxForwardedPorts {
		t.Fatalf("len(ExpandForwardedPorts(150 distinct single ports)) = %d, want %d", len(got), MaxForwardedPorts)
	}
}

func TestExceedsForwardedPortsCap(t *testing.T) {
	atCap := fmt.Sprintf("1-%d", MaxForwardedPorts)
	if ExceedsForwardedPortsCap(atCap) {
		t.Fatalf("a spec covering exactly %d ports is AT the cap, not over it", MaxForwardedPorts)
	}
	overCap := fmt.Sprintf("1-%d", MaxForwardedPorts+1)
	if !ExceedsForwardedPortsCap(overCap) {
		t.Fatalf("a spec covering %d ports must be reported as exceeding the cap", MaxForwardedPorts+1)
	}
	if ExceedsForwardedPortsCap("1-10") {
		t.Fatal("a small spec must not be reported as exceeding the cap")
	}
}
