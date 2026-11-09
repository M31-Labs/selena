attribute vec3 position;
attribute vec3 normal;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform vec3 edgeColor;
uniform float progress;
uniform float noiseScale;
uniform float edgeWidth;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
varying vec3 vWorldNormal;
varying vec3 vWorldPos;

void main() {
  vWorldNormal = normalize((normalMatrix * normal));
  vWorldPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
