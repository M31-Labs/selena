#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float cutoff;
in vec2 vUv;
out vec4 fragColor;

void main() {
  if ((vUv.x < cutoff)) {
    fragColor = vec4(vec3(1.0, 0.0, 0.0), 1.0); return;
  }
  fragColor = vec4(vec3(0.0, 1.0, 0.0), 1.0);
}
