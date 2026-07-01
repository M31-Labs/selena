#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
out vec4 fragColor;

void main() {
  fragColor = vec4(baseColor, 1.0);
}
