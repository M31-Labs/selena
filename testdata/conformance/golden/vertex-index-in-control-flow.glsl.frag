precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float gridSize;
varying float shade;

void main() {
  gl_FragColor = vec4(vec3(shade, shade, shade), 1.0);
}
