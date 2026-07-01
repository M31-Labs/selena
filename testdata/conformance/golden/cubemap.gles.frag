#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
in vec3 vPosition;
in vec3 vWorldNormal;
uniform samplerCube sky;
out vec4 fragColor;

void main() {
  vec3 dir = reflect((-vPosition), normalize(vWorldNormal));
  fragColor = vec4(texture(sky, dir).rgb, 1.0);
}
