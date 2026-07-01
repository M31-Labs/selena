#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 baseColor;
  float gain;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float3 vWorldNormal;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float3 n = normalize(in.vWorldNormal);
  float3 white = float3(1.0, 1.0, 1.0);
  float lit = max(n.y, 0.0);
  float acc = 0.0;
  if ((lit > 0.5)) {
    acc = u.gain;
  } else {
    acc = (u.gain * 0.5);
  }
  for (int i = 0; (i < 2); i = (i + 1)) {
    acc = (acc + 0.1);
  }
  return float4(((u.baseColor * (acc * lit)) + (white * 0.0)), 1.0);
}
