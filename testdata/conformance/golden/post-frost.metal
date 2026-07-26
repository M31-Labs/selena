#include <metal_stdlib>
using namespace metal;

struct UserUniforms {
  float blurLevel;
  float mixAmount;
  float4 rects[4];
};

struct PostOut {
  float4 pos [[position]];
  float2 v_uv;
};

vertex PostOut vertexMain(uint vid [[vertex_id]]) {
  const float2 pos[3] = { {-1,-1},{3,-1},{-1,3} };
  const float2 uvs[3] = { {0,1},{2,1},{0,-1} };
  PostOut out;
  out.pos  = float4(pos[vid], 0, 1);
  out.v_uv = uvs[vid];
  return out;
}

fragment float4 fragmentMain(PostOut in [[stage_in]], texture2d<float> _sceneColorTex [[texture(0)]], sampler _sceneColorSamp [[sampler(0)]], depth2d<float> _sceneDepthTex [[texture(2)]], sampler _sceneDepthSamp [[sampler(2)]], constant UserUniforms& u [[buffer(1)]]) {
  float2 size = float2(_sceneColorTex.get_width(), _sceneColorTex.get_height());
  float px = (1.0 / max(size.x, 1.0));
  float best = 1.0;
  for (int i = 0; (i < 4); i = (i + 1)) {
    float4 r = u.rects[i];
    float dx = (abs((in.v_uv.x - r.x)) - r.z);
    float dy = (abs((in.v_uv.y - r.y)) - r.w);
    float d = (length(float2(max(dx, 0.0), max(dy, 0.0))) + min(max(dx, dy), 0.0));
    best = min(best, d);
    if ((d < 0.0)) {
      break;
    }
  }
  float aa = max(fwidth(best), px);
  float inside = (1.0 - smoothstep((0.0 - aa), (0.0 + aa), best));
  float4 blurred = _sceneColorTex.sample(_sceneColorSamp, in.v_uv, level(u.blurLevel));
  float4 plain = _sceneColorTex.sample(_sceneColorSamp, in.v_uv);
  float k = (inside * u.mixAmount);
  return float4(mix(plain.r, blurred.r, k), mix(plain.g, blurred.g, k), mix(plain.b, blurred.b, k), plain.a);
}
