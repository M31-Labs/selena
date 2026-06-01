#version 300 es
precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float gain;
uniform vec2 offset;
uniform mat3 basis;
uniform mat4 tintMatrix;
out vec4 fragColor;

void main() {
  fragColor = vec4((baseColor * gain), 1.0);
}
