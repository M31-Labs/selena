#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 baseColor;
  char _selenaPad0[4];
  packed_float3 shadowColor;
  char _selenaPad1[4];
  packed_float3 outlineColor;
  float bands;
  float outlineWidth;
  char _selenaPad2[12];
  packed_float3 cameraPos;
  char _selenaPad3[4];
  packed_float3 sunDir;
  char _selenaPad4[4];
  packed_float3 sunColor;
  char _selenaPad5[4];
  packed_float3 ambient;
  char _selenaPad6[4];
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
  float3 l = normalize(float3(u.sunDir));
  float selenaLib0 = dot(n, l);
  float selenaLib1 = u.bands;
  float selenaLib2 = 0.08;
  float selenaLib3 = max(selenaLib1, 1.0);
  float selenaLib4 = (clamp(selenaLib0, 0.0, 1.0) * selenaLib3);
  float selenaLib5 = ((floor(selenaLib4) + smoothstep((1.0 - max(selenaLib2, 0.0001)), 1.0, fract(selenaLib4))) / selenaLib3);
  float light = selenaLib5;
  float3 body = (mix(float3(u.shadowColor), float3(u.baseColor), light) * (float3(u.sunColor) + float3(u.ambient)));
  float3 selenaLib6 = n;
  float3 selenaLib7 = v;
  float3 selenaLib8 = l;
  float selenaLib9 = 0.4;
  float selenaLib10 = 0.01;
  float selenaLib11 = (1.0 - (clamp(selenaLib9, 0.0, 1.0) * 0.1));
  float selenaLib12 = smoothstep((selenaLib11 - selenaLib10), (selenaLib11 + selenaLib10), dot(selenaLib6, normalize((selenaLib8 + selenaLib7))));
  float3 highlight = (selenaLib12 * float3(u.sunColor));
  float3 selenaLib13 = n;
  float3 selenaLib14 = v;
  float3 selenaLib15 = l;
  float selenaLib16 = 0.35;
  float selenaLib17 = 0.04;
  float selenaLib18 = ((1.0 - clamp(dot(selenaLib13, selenaLib14), 0.0, 1.0)) * clamp(((dot(selenaLib13, selenaLib15) * 0.5) + 0.5), 0.0, 1.0));
  float selenaLib19 = (1.0 - clamp(selenaLib16, 0.0, 1.0));
  float selenaLib20 = smoothstep((selenaLib19 - selenaLib17), (selenaLib19 + selenaLib17), selenaLib18);
  float rim = (selenaLib20 * 0.6);
  float3 shaded = ((body + highlight) + (float3(u.baseColor) * rim));
  float3 selenaLib21 = n;
  float3 selenaLib22 = v;
  float selenaLib23 = u.outlineWidth;
  float selenaLib24 = dot(selenaLib21, selenaLib22);
  float selenaLib25 = max((fwidth(selenaLib24) * max(selenaLib23, 0.0)), 0.0001);
  float selenaLib26 = (1.0 - smoothstep(selenaLib25, (selenaLib25 * 2.0), selenaLib24));
  return float4(mix(shaded, float3(u.outlineColor), selenaLib26), 1.0);
}
