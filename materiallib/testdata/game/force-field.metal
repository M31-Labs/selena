#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 tint;
  char _selenaPad0[4];
  packed_float3 hitColor;
  char _selenaPad1[4];
  packed_float3 hitPos;
  float hitAge;
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
  float3 selenaLib2 = ((dot(selenaLib0, selenaLib1) < 0.0) ? (-selenaLib0) : selenaLib0);
  float3 selenaLib3 = selenaLib2;
  float3 selenaLib4 = v;
  float3 selenaLib5 = in.vWorldPos;
  float selenaLib6 = u.time;
  float3 selenaLib7 = selenaLib3;
  float3 selenaLib8 = selenaLib4;
  float selenaLib9 = 3.0;
  float selenaLib10 = pow((1.0 - clamp(dot(selenaLib7, selenaLib8), 0.0, 1.0)), selenaLib9);
  float selenaLib11 = selenaLib10;
  float selenaLib12 = (selenaLib5.y + (selenaLib5.x * 0.3));
  float selenaLib13 = selenaLib6;
  float selenaLib14 = 3.0;
  float selenaLib15 = 0.4;
  float selenaLib16 = 4.0;
  float selenaLib17 = fract(((selenaLib12 * selenaLib14) - (selenaLib13 * selenaLib15)));
  float selenaLib18 = pow((1.0 - abs(((selenaLib17 * 2.0) - 1.0))), max(selenaLib16, 1.0));
  float selenaLib19 = (selenaLib18 * 0.35);
  float selenaLib20 = clamp((selenaLib11 + selenaLib19), 0.0, 2.0);
  float shell = selenaLib20;
  float3 selenaLib21 = in.vWorldPos;
  float3 selenaLib22 = float3(u.hitPos);
  float selenaLib23 = u.hitAge;
  float selenaLib24 = 3.0;
  float selenaLib25 = 0.25;
  float selenaLib26 = abs((distance(selenaLib21, selenaLib22) - (max(selenaLib23, 0.0) * selenaLib24)));
  float selenaLib27 = ((selenaLib23 >= 0.0) ? clamp((1.0 - selenaLib23), 0.0, 1.0) : 0.0);
  float selenaLib28 = ((1.0 - smoothstep(0.0, max(selenaLib25, 0.0001), selenaLib26)) * selenaLib27);
  float hit = selenaLib28;
  return float4(((float3(u.tint) * shell) + (float3(u.hitColor) * hit)), 1.0);
}
