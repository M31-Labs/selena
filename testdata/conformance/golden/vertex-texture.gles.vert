#version 300 es
in vec3 position;
in vec3 normal;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float scale;
uniform float lod;
uniform highp sampler2D heightMap;
out float height;
out vec2 vUv;

void main() {
  float h = textureLod(heightMap, uv, lod).x;
  vec3 p = (position + (normal * (h * scale)));
  height = h;
  vUv = uv;
  gl_Position = (mvp * vec4(p, 1.0));
}
