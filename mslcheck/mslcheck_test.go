package mslcheck

import (
	"runtime"
	"strings"
	"testing"
)

func TestFindIsStableAndSkipsOffDarwin(t *testing.T) {
	a, okA := Find()
	b, okB := Find()
	if a != b || okA != okB {
		t.Fatalf("Find is not stable: %+v %v, %+v %v", a, okA, b, okB)
	}
	if runtime.GOOS != "darwin" && okA {
		t.Fatalf("found a Metal compiler on %s: %+v", runtime.GOOS, a)
	}
	if okA && !strings.HasSuffix(a.Path, "/metal") {
		t.Fatalf("unexpected compiler path %q", a.Path)
	}
}

func TestCompileRejectsBadSourceOrSkips(t *testing.T) {
	good := "#include <metal_stdlib>\nusing namespace metal;\nfragment float4 fragmentMain() { return float4(1.0); }\n"
	res, err := Compile(good)
	if res.Skipped {
		t.Skip("no Metal compiler on this host")
	}
	if err != nil {
		t.Fatalf("valid MSL rejected: %v", err)
	}
	if _, err := Compile("fragment float4 fragmentMain() { return nope; }\n"); err == nil {
		t.Fatal("invalid MSL accepted")
	}
}
