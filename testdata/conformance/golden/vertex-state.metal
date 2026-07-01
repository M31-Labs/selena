#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
};

struct StateGrid {
  uint gridWidth;
  uint gridHeight;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float2 uv [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  VertexOut out;
  float4 s = _inState[min(uint((in.uv).x * float(_stateGrid.gridWidth)) + uint((in.uv).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float y = s.x;
  out.vUv = in.uv;
  out.position = (u.mvp * float4(in.position.x, y, in.position.z, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  float4 s = _inState[min(uint((in.vUv).x * float(_stateGrid.gridWidth)) + uint((in.vUv).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  return float4(float3(((s.x * 0.5) + 0.5), ((s.y * 0.5) + 0.5), ((s.z * 0.5) + 0.5)), 1.0);
}
