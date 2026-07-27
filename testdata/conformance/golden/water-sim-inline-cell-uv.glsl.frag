precision highp float;
varying vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;

void main() {
  vec4 here = texture2D(stateTex, vUV);
  float cx = (vUV).x;
  float cy = (vUV).y;
  float bump = ((sin((cx * 6.283185307179586)) * cos((cy * 6.283185307179586))) * 0.01);
  gl_FragColor = vec4((here.x + bump), here.y, here.z, here.w);
}
