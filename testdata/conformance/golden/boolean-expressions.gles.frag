#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
in vec2 vUv;
out vec4 fragColor;

void main() {
  bool parity = false;
  bool toggled = (parity != true);
  bool same = ((vUv.x < 0.5) == toggled);
  bool visible = ((!parity) && (same || false));
  fragColor = vec4((visible ? vec3(0.9, 0.7, 0.2) : vec3(0.1, 0.2, 0.4)), 1.0);
}
