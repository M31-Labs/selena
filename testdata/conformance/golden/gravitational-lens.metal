#include <metal_stdlib>
using namespace metal;

struct UserUniforms {
  float lensCenterX;
  float lensCenterY;
  float strength;
  float softening;
  float maxBend;
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
  float2 lensCenter = float2(u.lensCenterX, u.lensCenterY);
  float2 delta = (in.v_uv - lensCenter);
  float r2 = (dot(delta, delta) + (u.softening * u.softening));
  float bend = (u.strength / r2);
  float bendClamped = clamp(bend, 0.0, u.maxBend);
  float2 disp = (delta * bendClamped);
  float2 dispR = (disp * 1.15);
  float2 dispB = (disp * 0.85);
  float4 colorR = _sceneColorTex.sample(_sceneColorSamp, (in.v_uv - dispR));
  float4 colorG = _sceneColorTex.sample(_sceneColorSamp, (in.v_uv - disp));
  float4 colorB = _sceneColorTex.sample(_sceneColorSamp, (in.v_uv - dispB));
  float4 lensed = float4(colorR.r, colorG.g, colorB.b, 1.0);
  float4 rawColor = _sceneColorTex.sample(_sceneColorSamp, in.v_uv);
  float distCenter = length(delta);
  float blendEdge = smoothstep(0.0, (u.softening * 4.0), distCenter);
  return mix(rawColor, lensed, blendEdge);
}
