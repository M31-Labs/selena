#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform highp sampler2D stateTex;
in vec2 vUv;
out vec4 fragColor;

void main() {
  vec4 s = texture(stateTex, vUv);
  fragColor = vec4(vec3(((s.x * 0.5) + 0.5), ((s.y * 0.5) + 0.5), ((s.z * 0.5) + 0.5)), 1.0);
}
