package glsl

import (
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

func TestEmitDirectionalDiffuse(t *testing.T) {
	vert, frag, err := Emit(ir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"attribute vec3 position;",
		"uniform mat4 mvp;",
		"varying vec3 vNormal;",
		"vNormal = normalize((normalMatrix * normal));",
		"gl_Position = (mvp * vec4(position, 1.0));",
	} {
		if !strings.Contains(vert, want) {
			t.Errorf("GLSL vertex missing %q\n--- got ---\n%s", want, vert)
		}
	}
	for _, want := range []string{
		"precision mediump float;",
		"float diff = max(dot(n, normalize(lightDir)), 0.0);",
		"gl_FragColor = vec4(rgb, 1.0);",
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("GLSL fragment missing %q\n--- got ---\n%s", want, frag)
		}
	}
}
