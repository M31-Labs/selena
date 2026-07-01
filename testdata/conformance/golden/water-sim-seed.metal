#include <metal_stdlib>
using namespace metal;

struct GridUniforms {
  uint gridWidth;
  uint gridLen;
};

struct UserUniforms {
  float dropRadius;
  float dropCount;
  float4 drops[64];
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
  float h = here.x;
  int count = int(u.dropCount);
  for (int j = 0; (j < count); j = (j + 1)) {
    float4 drop = u.drops[j];
    float2 center = drop.xy;
    float r = max(u.dropRadius, 0.0001);
    float dist = max(0.0, (1.0 - (length((center - uv)) / r)));
    float bump = (0.5 - (cos((dist * 3.141592653589793)) * 0.5));
    h = (h + (bump * drop.w));
  }
  outState[cellIndex] = float4(h, here.y, here.z, here.w);
}
