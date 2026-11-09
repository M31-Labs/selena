precision highp float;
uniform float head;
uniform float gain;
varying vec2 v_uv;
uniform sampler2D spectrum;
uniform sampler2D palette;
uniform highp vec2 _texSize_spectrum;

void main() {
  vec2 sz = _texSize_spectrum;
  int col = int(mod(((v_uv.x * sz.x) + head), sz.x));
  int row = int(min((v_uv.y * sz.y), (sz.y - 1.0)));
  float mag = clamp((texture2D(spectrum, (vec2(col, row) + 0.5) / _texSize_spectrum).r * gain), 0.0, 1.0);
  vec4 c = texture2D(palette, vec2(mag, 0.5));
  gl_FragColor = vec4((c.r * mag), (c.g * mag), (c.b * mag), mag);
}
