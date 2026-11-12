#version 300 es
precision highp float;
precision highp int;
uniform float head;
uniform float gain;
in vec2 v_uv;
uniform sampler2D spectrum;
uniform sampler2D palette;
out vec4 fragColor;

void main() {
  vec2 sz = vec2(textureSize(spectrum, 0));
  int col = int(mod(((v_uv.x * sz.x) + head), sz.x));
  int row = int(min((v_uv.y * sz.y), (sz.y - 1.0)));
  float mag = clamp((texelFetch(spectrum, ivec2(col, row), 0).r * gain), 0.0, 1.0);
  vec4 c = texture(palette, vec2(mag, 0.5));
  fragColor = vec4((c.r * mag), (c.g * mag), (c.b * mag), mag);
}
