attribute vec3 position;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 shadow0;
uniform vec4 shadowMatrices0[16];
uniform highp sampler2D shadowMap0;
varying float vLift;
varying vec3 vPos;

void main() {
  vLift = texture2D(shadowMap0, uv).x;
  vPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
