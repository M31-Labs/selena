#include <metal_stdlib>
using namespace metal;

struct GridUniforms {
  uint gridWidth;
  uint gridLen;
};

kernel void computeMain(
  uint3 gid [[thread_position_in_grid]],
  const device float4* inState [[buffer(0)]],
  device float4* outState [[buffer(1)]],
  constant GridUniforms& _grid [[buffer(2)]]
) {
  uint cellIndex = gid.x;
  if (cellIndex >= _grid.gridLen) { return; }
  float4 here = inState[clamp(int(cellIndex) + (0) + (0) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)];
  float cx = ((float2(float(cellIndex % _grid.gridWidth), float(cellIndex / _grid.gridWidth)) + float2(0.5, 0.5)) / float(_grid.gridWidth)).x;
  float cy = ((float2(float(cellIndex % _grid.gridWidth), float(cellIndex / _grid.gridWidth)) + float2(0.5, 0.5)) / float(_grid.gridWidth)).y;
  float bump = ((sin((cx * 6.283185307179586)) * cos((cy * 6.283185307179586))) * 0.01);
  outState[cellIndex] = float4((here.x + bump), here.y, here.z, here.w);
}
