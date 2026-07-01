#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float a;
uniform float b;
out vec4 fragColor;

void main() {
  bool lt = (a < b);
  bool gt = (a > b);
  bool le = (a <= b);
  bool ge = (a >= b);
  bool eq = (a == b);
  bool ne = (a != b);
  bool both = (lt && gt);
  bool any = (le || ge);
  bool notv = (!eq);
  float sel = (ne ? a : b);
  fragColor = vec4(vec3(sel, 0.0, 0.0), 1.0);
}
