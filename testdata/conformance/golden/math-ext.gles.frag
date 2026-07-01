#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float ior;
in vec2 vUv;
in vec3 vWorldNormal;
out vec4 fragColor;

void main() {
  vec2 st = vUv;
  vec3 n = normalize(vWorldNormal);
  vec3 ri = refract(n, n, ior);
  float m = mod(st.x, 0.5);
  float rnd = round(st.y);
  float a1 = atan(st.x);
  float a2 = atan(st.y, st.x);
  float ai = asin(m);
  float ac = acos(m);
  float dx = dFdx(st.x);
  float dy = dFdy(st.y);
  float fw = fwidth(m);
  vec3 v3 = vec3(m, rnd, a1);
  vec4 v4 = vec4(st, ai, ac);
  fragColor = vec4(((ri + (v3 * (((a2 + dx) + dy) + fw))) + v4.xyz), 1.0);
}
