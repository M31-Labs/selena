#extension GL_OES_standard_derivatives : enable
#extension GL_EXT_shader_texture_lod : enable
precision mediump float;
varying vec2 v_uv;

uniform sampler2D _sceneColor;
uniform sampler2D _sceneDepth;
uniform vec2 _sceneSize;
uniform float blurLevel;
uniform float mixAmount;
uniform vec4 rects[4];

void main() {
  vec2 size = _sceneSize;
  float px = (1.0 / max(size.x, 1.0));
  float best = 1.0;
  for (int i = 0; (i < 4); i = (i + 1)) {
    vec4 r = rects[i];
    float dx = (abs((v_uv.x - r.x)) - r.z);
    float dy = (abs((v_uv.y - r.y)) - r.w);
    float d = (length(vec2(max(dx, 0.0), max(dy, 0.0))) + min(max(dx, dy), 0.0));
    best = min(best, d);
    if ((d < 0.0)) {
      break;
    }
  }
  float aa = max(fwidth(best), px);
  float inside = (1.0 - smoothstep((0.0 - aa), (0.0 + aa), best));
  vec4 blurred = texture2DLodEXT(_sceneColor, v_uv, blurLevel);
  vec4 plain = texture2D(_sceneColor, v_uv);
  float k = (inside * mixAmount);
  gl_FragColor = vec4(mix(plain.r, blurred.r, k), mix(plain.g, blurred.g, k), mix(plain.b, blurred.b, k), plain.a);
}
