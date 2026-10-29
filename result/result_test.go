package result

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The target names are wire values: GoSX keys its targetRequires descriptor by
// string(target), and the CLI maps its emit names onto these constants.
func TestTargetNames(t *testing.T) {
	for target, want := range map[Target]string{
		TargetWGSL:  "wgsl",
		TargetGLSL:  "glsl",
		TargetMetal: "metal",
		TargetGLES:  "gles",
	} {
		if string(target) != want {
			t.Errorf("target %q, want %q", target, want)
		}
	}
}

func TestAllTargetsOrderAndIsolation(t *testing.T) {
	want := []Target{TargetWGSL, TargetGLSL, TargetMetal, TargetGLES}
	got := AllTargets()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllTargets() = %v, want %v", got, want)
	}

	// A caller that edits the returned slice must not change later results.
	got[0] = Target("mutated")
	if again := AllTargets(); !reflect.DeepEqual(again, want) {
		t.Fatalf("AllTargets() after caller mutation = %v, want %v", again, want)
	}
}

func TestResultArtifact(t *testing.T) {
	res := Result{Artifacts: []Artifact{
		{Target: TargetWGSL, Source: "wgsl module"},
		{Target: TargetGLSL, Vertex: "glsl vertex", Fragment: "glsl fragment"},
		{Target: TargetWGSL, Source: "duplicate WGSL"},
	}}

	got, ok := res.Artifact(TargetGLSL)
	if !ok || got.Vertex != "glsl vertex" || got.Fragment != "glsl fragment" || got.Source != "" {
		t.Fatalf("Artifact(GLSL) = %+v, %v; want the split vertex/fragment artifact", got, ok)
	}

	// With repeated targets the first artifact wins.
	got, ok = res.Artifact(TargetWGSL)
	if !ok || got.Source != "wgsl module" {
		t.Fatalf("Artifact(WGSL) = %+v, %v; want the first WGSL artifact", got, ok)
	}

	for _, empty := range []Result{res, {}} {
		got, ok = empty.Artifact(TargetMetal)
		if ok || !reflect.DeepEqual(got, Artifact{}) {
			t.Fatalf("Artifact(Metal) = %+v, %v; want the zero Artifact and false", got, ok)
		}
	}
}

// Field names, field order and omitempty decide the bytes of every stored or
// transported artifact, so they are part of the compatibility contract.
func TestArtifactJSONShape(t *testing.T) {
	cases := []struct {
		name     string
		artifact Artifact
		want     string
	}{
		{
			name:     "source target",
			artifact: Artifact{Target: TargetWGSL, Source: "s"},
			want:     `{"Target":"wgsl","Source":"s","Vertex":"","Fragment":""}`,
		},
		{
			name:     "split target with specular variant",
			artifact: Artifact{Target: TargetGLSL, Vertex: "v", Fragment: "f", SpecularMRT: &FragmentArtifact{Source: "ms", Fragment: "mf"}},
			want:     `{"SpecularMRT":{"Source":"ms","Fragment":"mf"},"Target":"glsl","Source":"","Vertex":"v","Fragment":"f"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.artifact)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("JSON = %s\nwant   %s", got, tc.want)
			}
			var back Artifact
			if err := json.Unmarshal(got, &back); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(back, tc.artifact) {
				t.Fatalf("round trip = %+v, want %+v", back, tc.artifact)
			}
		})
	}
}

func TestResultJSONFieldOrder(t *testing.T) {
	data, err := json.Marshal(Result{})
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if _, err := dec.Token(); err != nil { // opening brace
		t.Fatal(err)
	}
	var keys []string
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key.(string))
		var skipped json.RawMessage
		if err := dec.Decode(&skipped); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"Program", "Material", "Module", "Layout", "Artifacts"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("Result JSON keys = %v, want %v", keys, want)
	}
}
