#version 300 es
in vec3 position;
in vec3 normal;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 tint;
uniform float density;
uniform float flicker;
uniform vec3 cameraPos;
uniform float time;
out vec3 vWorldNormal;
out vec3 vWorldPos;

void main() {
  vWorldNormal = normalize((normalMatrix * normal));
  vWorldPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
