#version 300 es
in vec3 position;
in vec3 normal;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float light_ambient;
uniform vec3 light_dir;
uniform float phase;
out vec2 vUv;
out vec3 vWorldNormal;

void main() {
  vUv = uv;
  vWorldNormal = normalize((normalMatrix * normal));
  gl_Position = (mvp * vec4(position, 1.0));
}
