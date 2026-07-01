precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float gridSize;
uniform float amplitude;
varying vec3 worldPos;

void main() {
  float h = ((worldPos.y * 0.5) + 0.5);
  gl_FragColor = vec4(vec3(h, ((worldPos.x * 0.5) + 0.5), ((worldPos.z * 0.5) + 0.5)), 1.0);
}
