#version 300 es
precision highp float;
in vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
out vec4 fragColor;

void main() {
  vec4 here = texture(stateTex, vUV);
  float h_east = texture(stateTex, vUV + vec2(1.0, 0.0) * texelSize).x;
  float h_north = texture(stateTex, vUV + vec2(0.0, 1.0) * texelSize).x;
  vec3 dx = vec3(1.0, (h_east - here.x), 0.0);
  vec3 dz = vec3(0.0, (h_north - here.x), 1.0);
  vec3 norm = normalize(cross(dz, dx));
  fragColor = vec4(here.x, here.y, norm.x, norm.z);
}
