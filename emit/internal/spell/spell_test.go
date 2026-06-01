package spell

import "testing"

func TestCallUsesBackendSpelling(t *testing.T) {
	got := Call("fract", []string{"x"}, Builtins{"fract": "fractf"})
	if got != "fractf(x)" {
		t.Fatalf("Call = %q, want fractf(x)", got)
	}
}

func TestCallFallsBackToSelenaName(t *testing.T) {
	got := Call("mix", []string{"a", "b", "t"}, nil)
	if got != "mix(a, b, t)" {
		t.Fatalf("Call = %q, want mix(a, b, t)", got)
	}
}
