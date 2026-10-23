#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float depthFade;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
  float2 uv [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]], depth2d<float> opaqueDepth [[texture(3)]], sampler opaqueDepthSampler [[sampler(3)]]) {
  VertexOut out;
  float d = opaqueDepth.sample(opaqueDepthSampler, in.uv, level(0.0));
  out.vUv = in.uv;
  out.position = (u.mvp * float4((in.position + (in.normal * (d * 0.01))), 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> bump [[texture(0)]], sampler bumpSampler [[sampler(0)]], texture2d<float> planarReflection [[texture(1)]], sampler planarReflectionSampler [[sampler(1)]], texture2d<float> opaqueColor [[texture(2)]], sampler opaqueColorSampler [[sampler(2)]], depth2d<float> opaqueDepth [[texture(3)]], sampler opaqueDepthSampler [[sampler(3)]]) {
  float b = bump.sample(bumpSampler, in.vUv).r;
  float3 refl = planarReflection.sample(planarReflectionSampler, (in.vUv + float2((b * 0.02), 0.0))).rgb;
  float3 under = opaqueColor.sample(opaqueColorSampler, in.vUv).rgb;
  float depth = opaqueDepth.sample(opaqueDepthSampler, in.vUv, level(0.0));
  float k = clamp((depth * u.depthFade), 0.0, 1.0);
  return float4(mix(under, refl, k), 1.0);
}
