attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float scale;
uniform float lod;
uniform highp sampler2D heightMap;
varying float height;
varying vec2 vUv;

void main() {
  float h = texture2DLod(heightMap, uv, lod).x;
  vec3 p = (position + (normal * (h * scale)));
  height = h;
  vUv = uv;
  gl_Position = (mvp * vec4(p, 1.0));
}
