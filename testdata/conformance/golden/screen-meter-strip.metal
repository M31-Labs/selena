#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float strips;
  char _selenaPad0[4];
  float2 screenSize;
  float4 levels[64];
  float4 holds[64];
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

fragment float4 fragmentMain(ScreenOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float x = (in.v_uv.x * u.strips);
  int i = int(min(floor(x), 63.0));
  float lane = fract(x);
  float gap = (u.strips / u.screenSize.x);
  float body = (step(gap, lane) * step(lane, (1.0 - gap)));
  float4 l = u.levels[i];
  float4 h = u.holds[i];
  float peak = ((lane < 0.5) ? l.x : l.y);
  float hold = ((lane < 0.5) ? h.x : h.y);
  float up = (1.0 - in.v_uv.y);
  float bar = step(up, ((peak + 60.0) / 60.0));
  float mark = (step(abs((up - ((hold + 60.0) / 60.0))), gap) * 0.5);
  float lit = (max(bar, mark) * body);
  return float4((lit * 0.2), lit, (lit * 0.4), lit);
}
