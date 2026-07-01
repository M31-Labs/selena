attribute vec3 position;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float threshold;
uniform vec3 colorA;
uniform vec3 colorB;
varying vec2 vUv;

void main() {
  vUv = uv;
  gl_Position = (mvp * vec4(position, 1.0));
}
