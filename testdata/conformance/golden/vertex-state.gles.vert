#version 300 es
in vec3 position;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform highp sampler2D stateTex;
out vec2 vUv;

void main() {
  vec4 s = textureLod(stateTex, uv, 0.0);
  float y = s.x;
  vUv = uv;
  gl_Position = (mvp * vec4(position.x, y, position.z, 1.0));
}
