precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
varying vec2 vUv;

void main() {
  bool parity = false;
  bool toggled = (parity != true);
  bool same = ((vUv.x < 0.5) == toggled);
  bool visible = ((!parity) && (same || false));
  gl_FragColor = vec4((visible ? vec3(0.9, 0.7, 0.2) : vec3(0.1, 0.2, 0.4)), 1.0);
}
