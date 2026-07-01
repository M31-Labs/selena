precision highp float;
varying vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;

void main() {
  vec4 here = texture2D(stateTex, vUV);
  float h_east = texture2D(stateTex, vUV + vec2(1.0, 0.0) * texelSize).x;
  float h_north = texture2D(stateTex, vUV + vec2(0.0, 1.0) * texelSize).x;
  vec3 dx = vec3(1.0, (h_east - here.x), 0.0);
  vec3 dz = vec3(0.0, (h_north - here.x), 1.0);
  vec3 norm = normalize(cross(dz, dx));
  gl_FragColor = vec4(here.x, here.y, norm.x, norm.z);
}
