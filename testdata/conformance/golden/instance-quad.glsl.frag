precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
varying vec4 vTone;

void main() {
  gl_FragColor = vec4(vec3(vTone.r, vTone.g, vTone.b), 1.0);
}
