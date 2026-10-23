#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float scale;
uniform float lod;
in float height;
in vec2 vUv;
uniform sampler2D heightMap;
uniform sampler2D tint;
out vec4 fragColor;

void main() {
  vec4 c = texture(tint, vUv);
  fragColor = vec4(vec3((c.x * (0.5 + height)), (c.y * (0.5 + height)), (c.z * (0.5 + height))), 1.0);
}
