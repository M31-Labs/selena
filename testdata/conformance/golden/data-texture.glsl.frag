precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float head;
varying float vLevel;
varying vec2 vUv;
uniform sampler2D spectrum;
uniform sampler2D palette;
uniform highp vec2 _texSize_spectrum;
uniform highp vec2 _texSize_palette;

void main() {
  vec2 sz = _texSize_spectrum;
  int col = int(mod(((vUv.x * sz.x) + head), sz.x));
  int row = int((vUv.y * sz.y));
  float mag = texture2D(spectrum, (vec2(col, row) + 0.5) / _texSize_spectrum).r;
  vec4 exact = texture2D(palette, (vec2(0, 0) + 0.5) / _texSize_palette);
  vec4 soft = texture2D(palette, vUv);
  gl_FragColor = vec4(vec3(((mag * exact.r) + (soft.r * 0.25)), ((mag * 0.6) + vLevel), (1.0 - mag)), 1.0);
}
