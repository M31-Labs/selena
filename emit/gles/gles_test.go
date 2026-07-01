package gles

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
		"#version 300 es",
		"in vec3 position;",
		"uniform mat4 mvp;",
		"out vec3 vNormal;",
		"gl_Position = (mvp * vec4(position, 1.0));",
	} {
		if !strings.Contains(vert, want) {
			t.Errorf("GLES vertex missing %q\n--- got ---\n%s", want, vert)
		}
	}
	for _, want := range []string{
		"#version 300 es",
		"precision highp float;",
		"in vec3 vNormal;",
		"out vec4 fragColor;",
		"float diff = max(dot(n, normalize(lightDir)), 0.0);",
		"fragColor = vec4(rgb, 1.0);",
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("GLES fragment missing %q\n--- got ---\n%s", want, frag)
		}
	}
}
