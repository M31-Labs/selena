#include <metal_stdlib>
using namespace metal;

struct GridUniforms {
  uint gridWidth;
  uint gridLen;
};

struct UserUniforms {
  float2 dropCenter;
  float dropRadius;
  float dropStrength;
};

kernel void computeMain(
  uint3 gid [[thread_position_in_grid]],
  const device float4* inState [[buffer(0)]],
  device float4* outState [[buffer(1)]],
  constant GridUniforms& _grid [[buffer(2)]],
  constant UserUniforms& u [[buffer(3)]]
) {
  uint cellIndex = gid.x;
  if (cellIndex >= _grid.gridLen) { return; }
  float4 here = inState[clamp(int(cellIndex) + (0) + (0) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)];
  float2 uv = (float2(float(cellIndex % _grid.gridWidth), float(cellIndex / _grid.gridWidth)) + float2(0.5, 0.5)) / float(_grid.gridWidth);
  float2 center = ((u.dropCenter * 0.5) + 0.5);
  float r = max(u.dropRadius, 0.0001);
  float d = max(0.0, (1.0 - (length((center - uv)) / r)));
  float bump = (0.5 - (cos((d * 3.141592653589793)) * 0.5));
  float h = (here.x + (bump * u.dropStrength));
  outState[cellIndex] = float4(h, here.y, here.z, here.w);
}
