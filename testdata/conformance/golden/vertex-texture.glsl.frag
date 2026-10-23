precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float scale;
uniform float lod;
varying float height;
varying vec2 vUv;
uniform sampler2D heightMap;
uniform sampler2D tint;

void main() {
  vec4 c = texture2D(tint, vUv);
  gl_FragColor = vec4(vec3((c.x * (0.5 + height)), (c.y * (0.5 + height)), (c.z * (0.5 + height))), 1.0);
}
