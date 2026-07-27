precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float cutoff;
varying vec2 vUv;

void main() {
  if ((vUv.x < cutoff)) {
    gl_FragColor = vec4(vec3(1.0, 0.0, 0.0), 1.0); return;
  }
  gl_FragColor = vec4(vec3(0.0, 1.0, 0.0), 1.0);
}
