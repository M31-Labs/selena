#version 300 es
in vec3 position;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 deepColor;
uniform vec3 skyColor;
uniform float waveScale;
uniform float choppy;
uniform vec3 cameraPos;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform float time;
out vec3 vWorldPos;

void main() {
  vWorldPos = position;
  gl_Position = (mvp * vec4(position, 1.0));
}
