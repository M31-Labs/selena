#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float limit;
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
  float hits = 0.0;
  for (float i = 0.0; (i < 8.0); i = (i + 1.0)) {
    if ((i > u.limit)) {
      return float4(float3(1.0, 0.0, 0.0), 1.0);
    }
    hits = (hits + 1.0);
    if ((hits > 6.0)) {
      break;
    }
  }
  return float4(float3(0.0, (hits / 8.0), 0.0), 1.0);
}
