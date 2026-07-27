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
  let cx = ((vec2<f32>(f32(cellIndex % _grid.gridWidth), f32(cellIndex / _grid.gridWidth)) + vec2<f32>(0.5, 0.5)) / f32(_grid.gridWidth)).x;
  let cy = ((vec2<f32>(f32(cellIndex % _grid.gridWidth), f32(cellIndex / _grid.gridWidth)) + vec2<f32>(0.5, 0.5)) / f32(_grid.gridWidth)).y;
  let bump = ((sin((cx * 6.283185307179586)) * cos((cy * 6.283185307179586))) * 0.01);
  outState[cellIndex] = vec4<f32>((here.x + bump), here.y, here.z, here.w);
}
