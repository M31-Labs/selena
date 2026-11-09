attribute vec3 position;
attribute vec3 normal;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 tint;
uniform vec3 hitColor;
uniform vec3 hitPos;
uniform float hitAge;
uniform vec3 cameraPos;
uniform float time;
varying vec3 vWorldNormal;
varying vec3 vWorldPos;

void main() {
  vWorldNormal = normalize((normalMatrix * normal));
  vWorldPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
