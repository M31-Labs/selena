#version 300 es
in vec3 position;
in vec3 normal;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 labelRect;
uniform vec4 numberRect;
uniform vec3 shirtColor;
uniform vec3 numberColor;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
out vec2 vUv;
out vec3 vWorldNormal;

void main() {
  vUv = uv;
  vWorldNormal = normalize((normalMatrix * normal));
  gl_Position = (mvp * vec4(position, 1.0));
}
