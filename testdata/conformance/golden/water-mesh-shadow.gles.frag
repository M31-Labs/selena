#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolHalfW;
uniform float poolHalfL;
uniform vec3 lightDir;
out vec4 fragColor;

void main() {
  fragColor = vec4(vec3(1.0, 1.0, 1.0), 1.0);
}
