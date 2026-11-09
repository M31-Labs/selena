#version 300 es
in vec3 position;
in vec3 normal;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 cameraPos;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
uniform vec4 fog;
uniform float exposure;
uniform vec3 tint;
out vec3 vPosition;
out vec3 vWorldNormal;

void main() {
  vPosition = position;
  vWorldNormal = normalize((normalMatrix * normal));
  gl_Position = (mvp * vec4(position, 1.0));
}
