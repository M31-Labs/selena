#version 300 es
in vec3 position;
in vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolHeight;
uniform vec4 baseColor;
uniform float isTexturePass;
uniform float texturePassMode;
uniform vec3 lightDir;
uniform highp sampler2D stateTex;
out vec3 worldPos;
out vec2 vUv;

void main() {
  worldPos = position;
  vUv = uv;
  gl_Position = (mvp * vec4(position, 1.0));
}
