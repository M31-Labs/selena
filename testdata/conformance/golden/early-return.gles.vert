#version 300 es
in vec3 position;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float cutoff;
out vec2 vUv;

void main() {
  vUv = uv;
  gl_Position = (mvp * vec4(position, 1.0));
}
