#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 baseColor;
};

struct VertexIn {
  float3 position [[attribute(0)]];
};

struct VertexOut {
  float4 position [[position]];
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  int n = 4;
  int acc = 0;
  for (int i = 0; (i < n); i = (i + 1)) {
    acc = (acc + 1);
  }
  float r = 0.0;
  float g = 0.0;
  if ((acc == n)) {
    r = u.baseColor.r;
  } else {
    g = u.baseColor.g;
  }
  float result = r;
  result = (result + g);
  return float4(float3(result, u.baseColor.g, u.baseColor.b), 1.0);
}
