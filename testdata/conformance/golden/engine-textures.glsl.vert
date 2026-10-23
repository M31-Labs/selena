attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float depthFade;
uniform highp sampler2D opaqueDepth;
varying vec2 vUv;

void main() {
  float d = texture2D(opaqueDepth, uv).x;
  vUv = uv;
  gl_Position = (mvp * vec4((position + (normal * (d * 0.01))), 1.0));
}
