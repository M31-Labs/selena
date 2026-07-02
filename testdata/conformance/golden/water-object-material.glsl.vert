attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolHeight;
uniform vec4 baseColor;
uniform float isTexturePass;
uniform float texturePassMode;
uniform vec3 lightDir;
uniform highp sampler2D stateTex;
varying vec3 worldPos;
varying vec2 vUv;
varying vec3 vNormal;

void main() {
  worldPos = position;
  vUv = uv;
  vNormal = normalize((normalMatrix * normal));
  gl_Position = (mvp * vec4(position, 1.0));
}
