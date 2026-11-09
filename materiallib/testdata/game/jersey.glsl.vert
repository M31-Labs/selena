attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 labelRect;
uniform vec4 numberRect;
uniform vec3 shirtColor;
uniform vec3 numberColor;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
varying vec2 vUv;
varying vec3 vWorldNormal;

void main() {
  vUv = uv;
  vWorldNormal = normalize((normalMatrix * normal));
  gl_Position = (mvp * vec4(position, 1.0));
}
