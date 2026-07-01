struct GridUniforms {
  gridWidth : u32,
  gridLen   : u32,
};
@group(0) @binding(0) var<uniform> _grid : GridUniforms;
@group(0) @binding(1) var<storage, read> inState : array<vec4<f32>>;
@group(0) @binding(2) var<storage, read_write> outState : array<vec4<f32>>;

@compute @workgroup_size(64)
fn computeMain(@builtin(global_invocation_id) gid : vec3<u32>) {
  let cellIndex = gid.x;
  if (cellIndex >= _grid.gridLen) { return; }
  let here = inState[clamp(i32(cellIndex) + (0) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)];
  let h_east = inState[clamp(i32(cellIndex) + (1) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)].x;
  let h_north = inState[clamp(i32(cellIndex) + (0) + (1) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)].x;
  let dx = vec3<f32>(1.0, (h_east - here.x), 0.0);
  let dz = vec3<f32>(0.0, (h_north - here.x), 1.0);
  let norm = normalize(cross(dz, dx));
  outState[cellIndex] = vec4<f32>(here.x, here.y, norm.x, norm.z);
}
