#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float scale;
  float lod;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
  float2 uv [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float height;
  float2 vUv;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> heightMap [[texture(0)]], sampler heightMapSampler [[sampler(0)]]) {
  VertexOut out;
  float h = heightMap.sample(heightMapSampler, in.uv, level(u.lod)).x;
  float3 p = (in.position + (in.normal * (h * u.scale)));
  out.height = h;
  out.vUv = in.uv;
  out.position = (u.mvp * float4(p, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> heightMap [[texture(0)]], sampler heightMapSampler [[sampler(0)]], texture2d<float> tint [[texture(1)]], sampler tintSampler [[sampler(1)]]) {
  float4 c = tint.sample(tintSampler, in.vUv);
  return float4(float3((c.x * (0.5 + in.height)), (c.y * (0.5 + in.height)), (c.z * (0.5 + in.height))), 1.0);
}
