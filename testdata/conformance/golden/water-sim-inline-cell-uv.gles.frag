#version 300 es
precision highp float;
in vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
out vec4 fragColor;

void main() {
  vec4 here = texture(stateTex, vUV);
  float cx = (vUV).x;
  float cy = (vUV).y;
  float bump = ((sin((cx * 6.283185307179586)) * cos((cy * 6.283185307179586))) * 0.01);
  fragColor = vec4((here.x + bump), here.y, here.z, here.w);
}
