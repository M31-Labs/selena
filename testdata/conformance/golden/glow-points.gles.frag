#version 300 es
precision mediump float;
in vec3 v_color;
in float v_alpha;
in float v_fogFactor;
in float v_pointSize;

uniform int u_hasFog;
uniform vec3 u_fogColor;
uniform vec3 fogColor;
out vec4 fragColor;

void main() {
  vec2 v_pointCoord = gl_PointCoord;
  vec2 centered = (v_pointCoord - vec2(0.5, 0.5));
  float radial = (length(centered) * 2.0);
  float sizeFocus = clamp(((v_pointSize - 4.0) / 48.0), 0.0, 1.0);
  float falloff = mix(4.2, 3.2, sizeFocus);
  float core = exp((-((radial * radial) * falloff)));
  float edge = (1.0 - smoothstep(0.78, 1.0, radial));
  float a = ((core * edge) * v_alpha);
  vec3 foggedRGB = mix(fogColor, v_color, v_fogFactor);
  fragColor = vec4(foggedRGB.r, foggedRGB.g, foggedRGB.b, a);
}
