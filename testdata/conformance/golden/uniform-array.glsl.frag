precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 tint;
uniform vec4 items[8];

void main() {
  vec4 acc = tint;
  for (int i = 0; (i < 8); i = (i + 1)) {
    vec4 item = items[i];
    acc = (acc + item);
  }
  gl_FragColor = vec4(acc.rgb, 1.0);
}
