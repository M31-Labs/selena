#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
in vec4 vTone;
out vec4 fragColor;

void main() {
  fragColor = vec4(vec3(vTone.r, vTone.g, vTone.b), 1.0);
}
