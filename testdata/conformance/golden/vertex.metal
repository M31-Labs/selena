#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float gridSize;
  float amplitude;
};

struct VertexOut {
  float4 position [[position]];
  float3 worldPos;
};

vertex VertexOut vertexMain(uint vertexIndex [[vertex_id]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  float fi = float(vertexIndex);
  float col = fract((fi / u.gridSize));
  float row = (floor((fi / u.gridSize)) / u.gridSize);
  float x = ((col * 2.0) - 1.0);
  float z = ((row * 2.0) - 1.0);
  float y = (sin((x * 6.2831855)) * u.amplitude);
  float3 p = float3(x, y, z);
  out.worldPos = p;
  out.position = (u.mvp * float4(x, y, z, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float h = ((in.worldPos.y * 0.5) + 0.5);
  return float4(float3(h, ((in.worldPos.x * 0.5) + 0.5), ((in.worldPos.z * 0.5) + 0.5)), 1.0);
}
