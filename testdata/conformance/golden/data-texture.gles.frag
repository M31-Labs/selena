#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float head;
in float vLevel;
in vec2 vUv;
uniform sampler2D spectrum;
uniform sampler2D gains;
uniform sampler2D palette;
out vec4 fragColor;

void main() {
  vec2 sz = vec2(textureSize(spectrum, 0));
  int col = int(mod(((vUv.x * sz.x) + head), sz.x));
  int row = int((vUv.y * sz.y));
  float mag = texelFetch(spectrum, ivec2(col, row), 0).r;
  vec4 exact = texelFetch(palette, ivec2(0, 0), 0);
  vec4 soft = texture(palette, vUv);
  fragColor = vec4(vec3(((mag * exact.r) + (soft.r * 0.25)), ((mag * 0.6) + vLevel), (1.0 - mag)), 1.0);
}
