#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float period;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float2 uv [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vUv = in.uv;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float angular = (((-1.25) + in.vUv.x) - (u.period * floor(((-1.25) + in.vUv.x) / u.period)));
  float2 vector = ((float2((-3.25), 5.25) + in.vUv) - (u.period * floor((float2((-3.25), 5.25) + in.vUv) / u.period)));
  return float4(float3(angular, vector.x, vector.y), 1.0);
}
