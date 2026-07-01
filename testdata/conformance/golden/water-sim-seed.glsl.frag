precision highp float;
varying vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
uniform float dropCount;
uniform float dropRadius;
uniform vec4 drops[64];

void main() {
  vec4 here = texture2D(stateTex, vUV);
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
  gl_FragColor = vec4(h, here.y, here.z, here.w);
}
