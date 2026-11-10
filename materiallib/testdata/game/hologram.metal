#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 tint;
  float density;
  float flicker;
  char _selenaPad0[12];
  packed_float3 cameraPos;
  float time;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float3 vWorldNormal;
  float3 vWorldPos;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.vWorldPos = in.position;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float3 n = normalize(in.vWorldNormal);
  float3 v = normalize((float3(u.cameraPos) - in.vWorldPos));
  float3 selenaLib0 = n;
  float3 selenaLib1 = v;
  float3 selenaLib2 = in.vWorldPos;
  float selenaLib3 = u.time;
  float selenaLib4 = u.density;
  float selenaLib5 = u.flicker;
  float3 selenaLib6 = selenaLib0;
  float3 selenaLib7 = selenaLib1;
  float selenaLib8 = 2.0;
  float selenaLib9 = pow((1.0 - clamp(dot(selenaLib6, selenaLib7), 0.0, 1.0)), selenaLib8);
  float selenaLib10 = selenaLib9;
  float selenaLib11 = selenaLib2.y;
  float selenaLib12 = selenaLib3;
  float selenaLib13 = selenaLib4;
  float selenaLib14 = 0.6;
  float selenaLib15 = 8.0;
  float selenaLib16 = fract(((selenaLib11 * selenaLib13) - (selenaLib12 * selenaLib14)));
  float selenaLib17 = pow((1.0 - abs(((selenaLib16 * 2.0) - 1.0))), max(selenaLib15, 1.0));
  float selenaLib18 = selenaLib17;
  float2 selenaLib19 = float2(floor((selenaLib3 * 24.0)), 7.0);
  float selenaLib20 = fract((sin(dot(selenaLib19, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib21 = (1.0 - (clamp(selenaLib5, 0.0, 1.0) * selenaLib20));
  float selenaLib22 = (((0.25 + (selenaLib10 * 0.9)) + (selenaLib18 * 0.5)) * selenaLib21);
  return float4((float3(u.tint) * selenaLib22), 1.0);
}
