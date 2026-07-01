#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 baseColor;
  float lo;
  float hi;
};

struct VertexIn {
  float3 position [[attribute(0)]];
};

struct VertexOut {
  float4 position [[position]];
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float3 clamped = clamp(u.baseColor, u.lo, u.hi);
  float3 bright = max(u.baseColor, u.lo);
  float3 dark = min(u.baseColor, u.hi);
  float3 blended = mix(u.baseColor, clamped, u.lo);
  float3 stepped = step(u.lo, u.baseColor);
  float3 powered = pow(clamped, u.hi);
  return float4((((((clamped + (bright * 0.0)) + (dark * 0.0)) + (blended * 0.0)) + (stepped * 0.0)) + (powered * 0.0)), 1.0);
}
