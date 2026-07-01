#version 300 es
precision highp float;
in vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
uniform float dropRadius;
uniform float dropCount;
uniform vec4 drops[64];
out vec4 fragColor;

void main() {
  vec4 here = texture(stateTex, vUV);
  vec2 uv = vUV;
  float h = here.x;
  int count = int(dropCount);
  for (int j = 0; (j < count); j = (j + 1)) {
    vec4 drop = drops[j];
    vec2 center = drop.xy;
    float r = max(dropRadius, 0.0001);
    float dist = max(0.0, (1.0 - (length((center - uv)) / r)));
    float bump = (0.5 - (cos((dist * 3.141592653589793)) * 0.5));
    h = (h + (bump * drop.w));
  }
  fragColor = vec4(h, here.y, here.z, here.w);
}
