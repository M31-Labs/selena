precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;

void main() {
  int n = 4;
  int acc = 0;
  for (int i = 0; (i < n); i = (i + 1)) {
    acc = (acc + 1);
  }
  float r = 0.0;
  float g = 0.0;
  if ((acc == n)) {
    r = baseColor.r;
  } else {
    g = baseColor.g;
  }
  float result = r;
  result = (result + g);
  gl_FragColor = vec4(vec3(result, baseColor.g, baseColor.b), 1.0);
}
