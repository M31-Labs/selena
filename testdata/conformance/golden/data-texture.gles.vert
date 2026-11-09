#version 300 es
in vec3 position;
in vec3 normal;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float head;
uniform highp sampler2D gains;
out float vLevel;
out vec2 vUv;

void main() {
  float columns = vec2(textureSize(gains, 0)).x;
  int column = int(clamp((uv.x * columns), 0.0, (columns - 1.0)));
  float level = texelFetch(gains, ivec2(column, 0), 0).r;
  vLevel = level;
  vUv = uv;
  gl_Position = (mvp * vec4((position + (normal * level)), 1.0));
}
