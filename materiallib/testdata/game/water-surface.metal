#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 deepColor;
  char _selenaPad0[4];
  packed_float3 skyColor;
  float waveScale;
  float choppy;
  char _selenaPad1[12];
  packed_float3 cameraPos;
  char _selenaPad2[4];
  packed_float3 sunDir;
  char _selenaPad3[4];
  packed_float3 sunColor;
  float time;
};

struct VertexIn {
  float3 position [[attribute(0)]];
};

struct VertexOut {
  float4 position [[position]];
  float3 vWorldPos;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vWorldPos = in.position;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float2 selenaLib0 = in.vWorldPos.xz;
  float selenaLib1 = u.time;
  float selenaLib2 = u.waveScale;
  float selenaLib3 = u.choppy;
  float2 selenaLib4 = (selenaLib0 * selenaLib2);
  float2 selenaLib5 = (float2(0.8, 0.6) * cos((dot(selenaLib4, float2(0.8, 0.6)) + (selenaLib1 * 1.3))));
  float2 selenaLib6 = (float2((-0.4), 0.9) * (cos(((dot(selenaLib4, float2((-0.4), 0.9)) * 1.7) + (selenaLib1 * 1.9))) * 0.6));
  float2 selenaLib7 = (float2(0.95, (-0.3)) * (cos(((dot(selenaLib4, float2(0.95, (-0.3))) * 2.9) + (selenaLib1 * 2.7))) * 0.35));
  float2 selenaLib8 = (((selenaLib5 + selenaLib6) + selenaLib7) * selenaLib3);
  float3 selenaLib9 = normalize(float3((-selenaLib8.x), 1.0, (-selenaLib8.y)));
  float3 n = selenaLib9;
  float3 v = normalize((float3(u.cameraPos) - in.vWorldPos));
  float3 l = normalize(float3(u.sunDir));
  float3 selenaLib10 = n;
  float3 selenaLib11 = v;
  float selenaLib12 = (1.0 - clamp(dot(selenaLib10, selenaLib11), 0.0, 1.0));
  float selenaLib13 = (selenaLib12 * selenaLib12);
  float selenaLib14 = (0.02 + (((0.98 * selenaLib13) * selenaLib13) * selenaLib12));
  float f = selenaLib14;
  float3 glint = ((pow(max(dot(reflect((-l), n), v), 0.0), 240.0) * float3(u.sunColor)) * 6.0);
  return float4((mix(float3(u.deepColor), float3(u.skyColor), f) + glint), 1.0);
}
