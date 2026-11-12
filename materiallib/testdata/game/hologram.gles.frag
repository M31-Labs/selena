#version 300 es
precision highp float;
precision highp int;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 tint;
uniform float density;
uniform float flicker;
uniform vec3 cameraPos;
uniform float time;
in vec3 vWorldNormal;
in vec3 vWorldPos;
out vec4 fragColor;

void main() {
  vec3 n = normalize(vWorldNormal);
  vec3 v = normalize((cameraPos - vWorldPos));
  vec3 selenaLib0 = n;
  vec3 selenaLib1 = v;
  vec3 selenaLib2 = vWorldPos;
  float selenaLib3 = time;
  float selenaLib4 = density;
  float selenaLib5 = flicker;
  vec3 selenaLib6 = selenaLib0;
  vec3 selenaLib7 = selenaLib1;
  float selenaLib8 = 2.0;
  float selenaLib9 = pow((1.0 - clamp(dot(selenaLib6, selenaLib7), 0.0, 1.0)), selenaLib8);
  float selenaLib10 = selenaLib9;
  float selenaLib11 = selenaLib2.y;
  float selenaLib12 = selenaLib3;
  float selenaLib13 = selenaLib4;
  float selenaLib14 = 0.6;
  float selenaLib15 = 8.0;
  float selenaLib16 = fract(((selenaLib11 * selenaLib13) - (selenaLib12 * selenaLib14)));
  float selenaLib17 = pow((1.0 - abs(((selenaLib16 * 2.0) - 1.0))), max(selenaLib15, 1.0));
  float selenaLib18 = selenaLib17;
  vec2 selenaLib19 = vec2(floor((selenaLib3 * 24.0)), 7.0);
  float selenaLib20 = fract((sin(dot(selenaLib19, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib21 = (1.0 - (clamp(selenaLib5, 0.0, 1.0) * selenaLib20));
  float selenaLib22 = (((0.25 + (selenaLib10 * 0.9)) + (selenaLib18 * 0.5)) * selenaLib21);
  fragColor = vec4((tint * selenaLib22), 1.0);
}
