#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float4 shadow0;
  float4 shadowMatrices0[16];
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float2 uv [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float vLift;
  float3 vPos;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]], depth2d_array<float> shadowMap0 [[texture(0)]], sampler shadowMap0Sampler [[sampler(0)]]) {
  VertexOut out;
  out.vLift = shadowMap0.sample(shadowMap0Sampler, in.uv, uint(0), level(0.0));
  out.vPos = in.position;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], depth2d_array<float> shadowMap0 [[texture(0)]], sampler shadowMap0Sampler [[sampler(0)]]) {
  int c = (int(u.shadow0.x) - 1);
  float4 s = ((u.shadowMatrices0[(c * 4)] * in.vPos.x) + u.shadowMatrices0[((c * 4) + 3)]);
  float lit = (((s.z - u.shadow0.y) <= shadowMap0.sample(shadowMap0Sampler, s.xy, uint(c), level(0.0))) ? 1.0 : 0.0);
  return float4(float3(lit, lit, in.vLift), 1.0);
}
