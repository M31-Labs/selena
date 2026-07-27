#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float gridSize;
in float shade;
out vec4 fragColor;

void main() {
  fragColor = vec4(vec3(shade, shade, shade), 1.0);
}
