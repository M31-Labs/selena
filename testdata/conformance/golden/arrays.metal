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
  float a[3];
  for (int i = 0; (i < 3); i = (i + 1)) {
    a[i] = 1.0;
  }
  a[0] = u.baseColor.r;
  a[1] = u.baseColor.g;
  a[2] = u.baseColor.b;
  float r = a[0];
  float g = a[1];
  float b = a[2];
  return float4(float3(r, g, b), 1.0);
}
