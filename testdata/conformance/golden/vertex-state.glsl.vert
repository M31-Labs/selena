attribute vec3 position;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform highp sampler2D stateTex;
varying vec2 vUv;

void main() {
  vec4 s = texture2DLod(stateTex, uv, 0.0);
  float y = s.x;
  vUv = uv;
  gl_Position = (mvp * vec4(position.x, y, position.z, 1.0));
}
