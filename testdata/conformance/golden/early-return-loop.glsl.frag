precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float limit;

void main() {
  float hits = 0.0;
  for (float i = 0.0; (i < 8.0); i = (i + 1.0)) {
    if ((i > limit)) {
      gl_FragColor = vec4(vec3(1.0, 0.0, 0.0), 1.0); return;
    }
    hits = (hits + 1.0);
    if ((hits > 6.0)) {
      break;
    }
  }
  gl_FragColor = vec4(vec3(0.0, (hits / 8.0), 0.0), 1.0);
}
