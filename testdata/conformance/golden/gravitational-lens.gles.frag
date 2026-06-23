#version 300 es
precision mediump float;
in vec2 v_uv;

uniform sampler2D _sceneColor;
uniform sampler2D _sceneDepth;
uniform float lensCenterX;
uniform float lensCenterY;
uniform float strength;
uniform float softening;
uniform float maxBend;
out vec4 fragColor;

void main() {
  vec2 lensCenter = vec2(lensCenterX, lensCenterY);
  vec2 delta = (v_uv - lensCenter);
  float r2 = (dot(delta, delta) + (softening * softening));
  float bend = (strength / r2);
  float bendClamped = clamp(bend, 0.0, maxBend);
  vec2 disp = (delta * bendClamped);
  vec2 dispR = (disp * 1.15);
  vec2 dispB = (disp * 0.85);
  vec4 colorR = texture(_sceneColor, (v_uv - dispR));
  vec4 colorG = texture(_sceneColor, (v_uv - disp));
  vec4 colorB = texture(_sceneColor, (v_uv - dispB));
  vec4 lensed = vec4(colorR.r, colorG.g, colorB.b, 1.0);
  vec4 rawColor = texture(_sceneColor, v_uv);
  float distCenter = length(delta);
  float blendEdge = smoothstep(0.0, (softening * 4.0), distCenter);
  fragColor = mix(rawColor, lensed, blendEdge);
}
