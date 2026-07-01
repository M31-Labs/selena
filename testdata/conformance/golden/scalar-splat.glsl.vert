attribute vec3 position;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float lo;
uniform float hi;

void main() {
  gl_Position = (mvp * vec4(position, 1.0));
}
