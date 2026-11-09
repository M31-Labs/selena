#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 shadow0;
uniform vec4 shadowMatrices0[16];
in float vLift;
in vec3 vPos;
uniform highp sampler2DArray shadowMap0;
out vec4 fragColor;

void main() {
  int c = (int(shadow0.x) - 1);
  vec4 s = ((shadowMatrices0[(c * 4)] * vPos.x) + shadowMatrices0[((c * 4) + 3)]);
  float lit = (((s.z - shadow0.y) <= textureLod(shadowMap0, vec3(s.xy, float(c)), 0.0).x) ? 1.0 : 0.0);
  fragColor = vec4(vec3(lit, lit, vLift), 1.0);
}
