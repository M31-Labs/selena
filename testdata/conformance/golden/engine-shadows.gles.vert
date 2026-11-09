#version 300 es
in vec3 position;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 shadow0;
uniform vec4 shadowMatrices0[16];
uniform highp sampler2DArray shadowMap0;
out float vLift;
out vec3 vPos;

void main() {
  vLift = textureLod(shadowMap0, vec3(uv, float(0)), 0.0).x;
  vPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
