#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
};

struct VertexIn {
  float4 rect [[attribute(0)]];
  float4 tone [[attribute(1)]];
  float spin [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float4 vTone;
};

vertex VertexOut vertexMain(uint vertexIndex [[vertex_id]], VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  float fi = float(vertexIndex);
  float2 corner = float2((fract((fi * 0.5)) * 2.0), floor((fi * 0.5)));
  float c = cos(in.spin);
  float s = sin(in.spin);
  float2 centered = (corner - float2(0.5, 0.5));
  float2 turned = float2(((centered.x * c) - (centered.y * s)), ((centered.x * s) + (centered.y * c)));
  float2 p = (in.rect.xy + ((turned + float2(0.5, 0.5)) * in.rect.zw));
  out.vTone = in.tone;
  out.position = (u.mvp * float4(p, 0.0, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  return float4(float3(in.vTone.r, in.vTone.g, in.vTone.b), 1.0);
}
