attribute vec3 position;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 lightDir;
uniform float poolHeight;
uniform vec4 baseColor;
uniform float isTexturePass;
uniform float texturePassMode;
uniform highp sampler2D stateTex;
varying vec3 worldPos;
varying vec2 vUv;

void main() {
  worldPos = position;
  vUv = uv;
  gl_Position = (mvp * vec4(position, 1.0));
}
