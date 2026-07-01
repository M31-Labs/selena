#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
out vec4 fragColor;

void main() {
  float a[3];
  for (int i = 0; (i < 3); i = (i + 1)) {
    a[i] = 1.0;
  }
  a[0] = baseColor.r;
  a[1] = baseColor.g;
  a[2] = baseColor.b;
  float r = a[0];
  float g = a[1];
  float b = a[2];
  fragColor = vec4(vec3(r, g, b), 1.0);
}
