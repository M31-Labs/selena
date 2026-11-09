#ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
precision highp int;
#else
precision mediump float;
precision mediump int;
#endif
uniform float strips;
uniform vec2 screenSize;
uniform vec4 levels[64];
uniform vec4 holds[64];
varying vec2 v_uv;

void main() {
  float x = (v_uv.x * strips);
  int i = int(min(floor(x), 63.0));
  float lane = fract(x);
  float gap = (strips / screenSize.x);
  float body = (step(gap, lane) * step(lane, (1.0 - gap)));
  vec4 l = levels[i];
  vec4 h = holds[i];
  float peak = ((lane < 0.5) ? l.x : l.y);
  float hold = ((lane < 0.5) ? h.x : h.y);
  float up = (1.0 - v_uv.y);
  float bar = step(up, ((peak + 60.0) / 60.0));
  float mark = (step(abs((up - ((hold + 60.0) / 60.0))), gap) * 0.5);
  float lit = (max(bar, mark) * body);
  gl_FragColor = vec4((lit * 0.2), lit, (lit * 0.4), lit);
}
