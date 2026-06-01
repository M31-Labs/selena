precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;

void main() {
  gl_FragColor = vec4(baseColor, 1.0);
}
