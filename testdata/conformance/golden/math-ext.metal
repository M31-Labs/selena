#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float ior;
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

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float2 st = in.vUv;
  float3 n = normalize(in.vWorldNormal);
  float3 ri = refract(n, n, u.ior);
  float m = fmod(st.x, 0.5);
  float rnd = round(st.y);
  float a1 = atan(st.x);
  float a2 = atan2(st.y, st.x);
  float ai = asin(m);
  float ac = acos(m);
  float dx = dfdx(st.x);
  float dy = dfdy(st.y);
  float fw = fwidth(m);
  float3 v3 = float3(m, rnd, a1);
  float4 v4 = float4(st, ai, ac);
  return float4(((ri + (v3 * (((a2 + dx) + dy) + fw))) + v4.xyz), 1.0);
}
