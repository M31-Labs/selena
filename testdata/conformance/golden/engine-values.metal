#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 cameraPos;
  float3 sunDir;
  float3 sunColor;
  float3 ambient;
  float4 fog;
  float exposure;
  float3 tint;
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
  float3 v = normalize((u.cameraPos - in.vPosition));
  float diffuse = max(dot(n, u.sunDir), 0.0);
  float3 lit = (((u.ambient + (u.sunColor * diffuse)) * u.tint) * u.exposure);
  float dist = length((u.cameraPos - in.vPosition));
  float f = (1.0 - exp(((-u.fog.w) * dist)));
  return float4((mix(lit, u.fog.xyz, f) * (0.5 + (0.5 * dot(n, v)))), 1.0);
}
