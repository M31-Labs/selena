attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float light_ambient;
uniform vec3 light_dir;
uniform float phase;
varying vec2 vUv;
varying vec3 vWorldNormal;

void main() {
  vUv = uv;
  vWorldNormal = normalize((normalMatrix * normal));
  gl_Position = (mvp * vec4(position, 1.0));
}
