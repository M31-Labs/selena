precision highp float;
varying vec2 vUV;

uniform highp sampler2D stateTex;
uniform vec2 texelSize;
uniform float damping;

void main() {
  vec4 here = texture2D(stateTex, vUV);
  vec4 avg = ((((texture2D(stateTex, vUV + vec2(1.0, 0.0) * texelSize) + texture2D(stateTex, vUV + vec2(-1.0, 0.0) * texelSize)) + texture2D(stateTex, vUV + vec2(0.0, 1.0) * texelSize)) + texture2D(stateTex, vUV + vec2(0.0, -1.0) * texelSize)) * 0.25);
  float vel = ((here.y + (avg.x - here.x)) * damping);
  float h = (here.x + vel);
  gl_FragColor = vec4(h, vel, here.z, here.w);
}
