precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 shadow0;
uniform vec4 shadowMatrices0[16];
varying float vLift;
varying vec3 vPos;
uniform highp sampler2D shadowMap0;

void main() {
  int c = (int(shadow0.x) - 1);
  vec4 s = ((shadowMatrices0[(c * 4)] * vPos.x) + shadowMatrices0[((c * 4) + 3)]);
  float lit = (((s.z - shadow0.y) <= texture2D(shadowMap0, s.xy).x) ? 1.0 : 0.0);
  gl_FragColor = vec4(vec3(lit, lit, vLift), 1.0);
}
