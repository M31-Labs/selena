#version 300 es
precision highp float;
precision highp int;
uniform mat4 mvp;
uniform mat3 normalMatrix;
in vec4 vTone;
flat in int vRegion;
out vec4 fragColor;

void main() {
  float column = float((vRegion & 3));
  float lift = (0.25 * column);
  fragColor = vec4(vec3((vTone.r + lift), vTone.g, vTone.b), 1.0);
}
