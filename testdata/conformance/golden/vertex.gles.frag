#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float gridSize;
uniform float amplitude;
in vec3 worldPos;
out vec4 fragColor;

void main() {
  float h = ((worldPos.y * 0.5) + 0.5);
  fragColor = vec4(vec3(h, ((worldPos.x * 0.5) + 0.5), ((worldPos.z * 0.5) + 0.5)), 1.0);
}
