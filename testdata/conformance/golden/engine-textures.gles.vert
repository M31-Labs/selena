#version 300 es
in vec3 position;
in vec3 normal;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float depthFade;
uniform highp sampler2D opaqueDepth;
out vec2 vUv;

void main() {
  float d = textureLod(opaqueDepth, uv, 0.0).x;
  vUv = uv;
  gl_Position = (mvp * vec4((position + (normal * (d * 0.01))), 1.0));
}
