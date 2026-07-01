#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
out vec4 fragColor;

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
  fragColor = vec4(vec3(result, baseColor.g, baseColor.b), 1.0);
}
