#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 cameraPos;
  char _selenaPad0[4];
  packed_float3 sunDir;
  char _selenaPad1[4];
  packed_float3 sunColor;
  char _selenaPad2[4];
  packed_float3 ambient;
  char _selenaPad3[4];
  float4 fog;
  float exposure;
  char _selenaPad4[12];
  packed_float3 tint;
  char _selenaPad5[4];
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float3 vPosition;
  float3 vWorldNormal;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vPosition = in.position;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float3 n = normalize(in.vWorldNormal);
  float3 v = normalize((float3(u.cameraPos) - in.vPosition));
  float diffuse = max(dot(n, float3(u.sunDir)), 0.0);
  float3 lit = (((float3(u.ambient) + (float3(u.sunColor) * diffuse)) * float3(u.tint)) * u.exposure);
  float dist = length((float3(u.cameraPos) - in.vPosition));
  float f = (1.0 - exp(((-u.fog.w) * dist)));
  return float4((mix(lit, u.fog.xyz, f) * (0.5 + (0.5 * dot(n, v)))), 1.0);
}
