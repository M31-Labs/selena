#include <metal_stdlib>
using namespace metal;

struct GridUniforms {
  uint gridWidth;
  uint gridLen;
};

struct UserUniforms {
  float damping;
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
  float4 avg = ((((inState[clamp(int(cellIndex) + (1) + (0) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)] + inState[clamp(int(cellIndex) + (-1) + (0) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)]) + inState[clamp(int(cellIndex) + (0) + (1) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)]) + inState[clamp(int(cellIndex) + (0) + (-1) * int(_grid.gridWidth), 0, int(_grid.gridLen) - 1)]) * 0.25);
  float vel = ((here.y + (avg.x - here.x)) * u.damping);
  float h = (here.x + vel);
  outState[cellIndex] = float4(h, vel, here.z, here.w);
}
