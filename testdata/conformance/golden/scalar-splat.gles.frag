#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float lo;
uniform float hi;
out vec4 fragColor;

void main() {
  vec3 clamped = clamp(baseColor, lo, hi);
  vec3 bright = max(baseColor, lo);
  vec3 dark = min(baseColor, hi);
  vec3 blended = mix(baseColor, clamped, lo);
  vec3 stepped = step(lo, baseColor);
  vec3 powered = pow(clamped, vec3(hi));
  fragColor = vec4((((((clamped + (bright * 0.0)) + (dark * 0.0)) + (blended * 0.0)) + (stepped * 0.0)) + (powered * 0.0)), 1.0);
}
