#version 300 es
in vec3 position;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 tint;
uniform vec4 items[8];

void main() {
  gl_Position = (mvp * vec4(position, 1.0));
}
