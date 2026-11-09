#version 300 es
in vec3 position;
in vec3 normal;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 albedo;
uniform vec3 scatter;
uniform float wrap;
uniform float thickness;
uniform vec3 cameraPos;
uniform vec3 ambient;
uniform vec4 fog;
uniform float lightCount;
uniform vec4 lights[32];
out vec3 vWorldNormal;
out vec3 vWorldPos;

void main() {
  vWorldNormal = normalize((normalMatrix * normal));
  vWorldPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
