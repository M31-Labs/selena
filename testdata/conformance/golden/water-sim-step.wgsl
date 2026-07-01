struct GridUniforms {
  gridWidth : u32,
  gridLen   : u32,
};
@group(0) @binding(0) var<uniform> _grid : GridUniforms;
@group(0) @binding(1) var<storage, read> inState : array<vec4<f32>>;
@group(0) @binding(2) var<storage, read_write> outState : array<vec4<f32>>;

struct UserUniforms {
  waveSpeed : f32,
  damping : f32,
};
@group(0) @binding(3) var<uniform> u : UserUniforms;

@compute @workgroup_size(64)
fn computeMain(@builtin(global_invocation_id) gid : vec3<u32>) {
  let cellIndex = gid.x;
  if (cellIndex >= _grid.gridLen) { return; }
  let here = inState[clamp(i32(cellIndex) + (0) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)];
  let avg = ((((inState[clamp(i32(cellIndex) + (1) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)] + inState[clamp(i32(cellIndex) + (-1) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)]) + inState[clamp(i32(cellIndex) + (0) + (1) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)]) + inState[clamp(i32(cellIndex) + (0) + (-1) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)]) * 0.25);
  let vel = ((here.y + (((avg.x - here.x) * 2.0) * u.waveSpeed)) * u.damping);
  let h = (here.x + vel);
  outState[cellIndex] = vec4<f32>(h, vel, here.z, here.w);
}
