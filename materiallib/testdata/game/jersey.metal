#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float4 labelRect;
  float4 numberRect;
  packed_float3 shirtColor;
  char _selenaPad0[4];
  packed_float3 numberColor;
  char _selenaPad1[4];
  packed_float3 sunDir;
  char _selenaPad2[4];
  packed_float3 sunColor;
  char _selenaPad3[4];
  packed_float3 ambient;
  char _selenaPad4[4];
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
  float2 uv [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
  float3 vWorldNormal;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vUv = in.uv;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> labels [[texture(0)]], sampler labelsSampler [[sampler(0)]]) {
  float2 selenaLib0 = in.vUv;
  float4 selenaLib1 = u.numberRect;
  float2 selenaLib2 = ((selenaLib0 - selenaLib1.xy) / max((selenaLib1.zw - selenaLib1.xy), float2(0.000001, 0.000001)));
  float2 selenaLib3 = selenaLib2;
  float4 selenaLib4 = u.labelRect;
  float2 selenaLib5 = mix(selenaLib4.xy, selenaLib4.zw, clamp(selenaLib3, float2(0.0, 0.0), float2(1.0, 1.0)));
  float4 glyph = labels.sample(labelsSampler, selenaLib5);
  float2 selenaLib6 = in.vUv;
  float4 selenaLib7 = u.numberRect;
  float2 selenaLib8 = selenaLib6;
  float4 selenaLib9 = selenaLib7;
  float2 selenaLib10 = ((selenaLib8 - selenaLib9.xy) / max((selenaLib9.zw - selenaLib9.xy), float2(0.000001, 0.000001)));
  float2 selenaLib11 = selenaLib10;
  float selenaLib12 = (((step(0.0, selenaLib11.x) * step(selenaLib11.x, 1.0)) * step(0.0, selenaLib11.y)) * step(selenaLib11.y, 1.0));
  float ink = (glyph.a * selenaLib12);
  float3 n = normalize(in.vWorldNormal);
  float3 light = (float3(u.ambient) + (float3(u.sunColor) * max(dot(n, normalize(float3(u.sunDir))), 0.0)));
  return float4((mix(float3(u.shirtColor), (float3(u.numberColor) * glyph.rgb), ink) * light), 1.0);
}
