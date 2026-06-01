#version 300 es
in vec3 position;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float gain;
uniform vec2 offset;
uniform mat3 basis;
uniform mat4 tintMatrix;

void main() {
  gl_Position = (mvp * vec4(position, 1.0));
}
