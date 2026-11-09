#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float head;
  float gain;
};

struct ScreenOut {
  float4 position [[position]];
  float2 v_uv;
};

vertex ScreenOut vertexMain(uint vid [[vertex_id]]) {
  const float2 pos[3] = { {-1,-1},{3,-1},{-1,3} };
  const float2 uvs[3] = { {0,1},{2,1},{0,-1} };
  ScreenOut out;
  out.position = float4(pos[vid], 0, 1);
  out.v_uv     = uvs[vid];
  return out;
}

fragment float4 fragmentMain(ScreenOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> spectrum [[texture(0)]], sampler spectrumSampler [[sampler(0)]], texture2d<float> palette [[texture(1)]], sampler paletteSampler [[sampler(1)]]) {
  float2 sz = float2(spectrum.get_width(), spectrum.get_height());
  int col = int((((in.v_uv.x * sz.x) + u.head) - (sz.x * floor(((in.v_uv.x * sz.x) + u.head) / sz.x))));
  int row = int(min((in.v_uv.y * sz.y), (sz.y - 1.0)));
  float mag = clamp((spectrum.read(uint2(col, row), 0).r * u.gain), 0.0, 1.0);
  float4 c = palette.sample(paletteSampler, float2(mag, 0.5));
  return float4((c.r * mag), (c.g * mag), (c.b * mag), mag);
}
