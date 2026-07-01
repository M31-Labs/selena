#version 300 es
precision highp float;
in vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
uniform float damping;
out vec4 fragColor;

void main() {
  vec4 here = texture(stateTex, vUV);
  vec4 avg = ((((texture(stateTex, vUV + vec2(1.0, 0.0) * texelSize) + texture(stateTex, vUV + vec2(-1.0, 0.0) * texelSize)) + texture(stateTex, vUV + vec2(0.0, 1.0) * texelSize)) + texture(stateTex, vUV + vec2(0.0, -1.0) * texelSize)) * 0.25);
  float vel = ((here.y + (avg.x - here.x)) * damping);
  float h = (here.x + vel);
  fragColor = vec4(h, vel, here.z, here.w);
}
