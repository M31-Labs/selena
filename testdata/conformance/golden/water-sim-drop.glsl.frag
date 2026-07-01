precision highp float;
varying vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
uniform vec2 dropCenter;
uniform float dropRadius;
uniform float dropStrength;

void main() {
  vec4 here = texture2D(stateTex, vUV);
  vec2 uv = vUV;
  vec2 center = ((dropCenter * 0.5) + 0.5);
  float r = max(dropRadius, 0.0001);
  float d = max(0.0, (1.0 - (length((center - uv)) / r)));
  float bump = (0.5 - (cos((d * 3.141592653589793)) * 0.5));
  float h = (here.x + (bump * dropStrength));
  gl_FragColor = vec4(h, here.y, here.z, here.w);
}
