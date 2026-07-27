attribute vec3 position;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float limit;

void main() {
  gl_Position = (mvp * vec4(position, 1.0));
}
