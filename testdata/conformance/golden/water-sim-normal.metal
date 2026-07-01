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
  float h_east = inState[clamp(int(cellIndex) + (1) + (0) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)].x;
  float h_north = inState[clamp(int(cellIndex) + (0) + (1) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)].x;
  float3 dx = float3(1.0, (h_east - here.x), 0.0);
  float3 dz = float3(0.0, (h_north - here.x), 1.0);
  float3 norm = normalize(cross(dz, dx));
  outState[cellIndex] = float4(here.x, here.y, norm.x, norm.z);
}
