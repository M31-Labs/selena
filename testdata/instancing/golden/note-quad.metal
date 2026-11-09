#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
};

struct VertexIn {
  float4 rect [[attribute(0)]];
  float4 tone [[attribute(1)]];
  int region [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float4 vTone;
  int vRegion [[flat]];
};

vertex VertexOut vertexMain(uint vertexIndex [[vertex_id]], uint instanceIndex [[instance_id]], VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  float2 corner = float2(float((vertexIndex & 1u)), float((vertexIndex >> 1u)));
  float2 p = (in.rect.xy + (corner * in.rect.zw));
  float stripe = (1.0 - (0.04 * float((instanceIndex & 3u))));
  out.vTone = float4((in.tone.rgb * stripe), in.tone.a);
  out.vRegion = in.region;
  out.position = (u.mvp * float4(p, 0.0, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float column = float((in.vRegion & 3));
  float lift = (0.25 * column);
  return float4(float3((in.vTone.r + lift), in.vTone.g, in.vTone.b), 1.0);
}
