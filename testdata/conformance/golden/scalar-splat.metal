#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 baseColor;
  float lo;
  float hi;
  char _selenaPad0[12];
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
  float3 clamped = clamp(float3(u.baseColor), u.lo, u.hi);
  float3 bright = max(float3(u.baseColor), u.lo);
  float3 dark = min(float3(u.baseColor), u.hi);
  float3 blended = mix(float3(u.baseColor), clamped, u.lo);
  float3 stepped = step(u.lo, float3(u.baseColor));
  float3 powered = pow(clamped, u.hi);
  return float4((((((clamped + (bright * 0.0)) + (dark * 0.0)) + (blended * 0.0)) + (stepped * 0.0)) + (powered * 0.0)), 1.0);
}
