#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
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
  bool parity = false;
  bool toggled = (parity != true);
  bool same = ((in.vUv.x < 0.5) == toggled);
  bool visible = ((!parity) && (same || false));
  return float4((visible ? float3(0.9, 0.7, 0.2) : float3(0.1, 0.2, 0.4)), 1.0);
}
