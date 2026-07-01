#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float threshold;
uniform vec3 colorA;
uniform vec3 colorB;
in vec2 vUv;
out vec4 fragColor;

void main() {
  float su = vUv.x;
  float sv = vUv.y;
  float band = ((su > threshold) ? 1.0 : 0.0);
  float pick = ((su > 0.5) ? ((sv > 0.5) ? 1.0 : 0.5) : 0.0);
  float pick2 = ((su > 0.5) ? 0.25 : ((sv > 0.5) ? 0.75 : 1.0));
  float m = clamp(((su > threshold) ? band : pick), 0.0, 1.0);
  float shade = ((((m > 0.5) ? pick : pick2) * band) + (1.0 - m));
  fragColor = vec4(((colorA * shade) + (colorB * (1.0 - shade))), 1.0);
}
