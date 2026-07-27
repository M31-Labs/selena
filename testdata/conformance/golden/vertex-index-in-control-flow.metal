#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float gridSize;
};

struct VertexOut {
  float4 position [[position]];
  float shade;
};

vertex VertexOut vertexMain(uint vertexIndex [[vertex_id]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  float fi = 0.0;
  if ((u.gridSize > 0.0)) {
    fi = float(vertexIndex);
  }
  float col = fract((fi / u.gridSize));
  float row = (floor((fi / u.gridSize)) / u.gridSize);
  out.shade = col;
  out.position = (u.mvp * float4(((col * 2.0) - 1.0), 0.0, ((row * 2.0) - 1.0), 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  return float4(float3(in.shade, in.shade, in.shade), 1.0);
}
