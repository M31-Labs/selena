#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float threshold;
  float3 colorA;
  float3 colorB;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float2 uv [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vUv = in.uv;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float su = in.vUv.x;
  float sv = in.vUv.y;
  float band = ((su > u.threshold) ? 1.0 : 0.0);
  float pick = ((su > 0.5) ? ((sv > 0.5) ? 1.0 : 0.5) : 0.0);
  float pick2 = ((su > 0.5) ? 0.25 : ((sv > 0.5) ? 0.75 : 1.0));
  float m = clamp(((su > u.threshold) ? band : pick), 0.0, 1.0);
  float shade = ((((m > 0.5) ? pick : pick2) * band) + (1.0 - m));
  return float4(((u.colorA * shade) + (u.colorB * (1.0 - shade))), 1.0);
}
