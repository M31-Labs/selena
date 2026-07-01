struct GridUniforms {
  gridWidth : u32,
  gridLen   : u32,
};
@group(0) @binding(0) var<uniform> _grid : GridUniforms;
@group(0) @binding(1) var<storage, read> inState : array<vec4<f32>>;
@group(0) @binding(2) var<storage, read_write> outState : array<vec4<f32>>;

struct UserUniforms {
  dropCenter : vec2<f32>,
  dropRadius : f32,
  dropStrength : f32,
};
@group(0) @binding(3) var<uniform> u : UserUniforms;

@compute @workgroup_size(64)
fn computeMain(@builtin(global_invocation_id) gid : vec3<u32>) {
  let cellIndex = gid.x;
  if (cellIndex >= _grid.gridLen) { return; }
  let here = inState[clamp(i32(cellIndex) + (0) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)];
  let uv = (vec2<f32>(f32(cellIndex % _grid.gridWidth), f32(cellIndex / _grid.gridWidth)) + vec2<f32>(0.5, 0.5)) / f32(_grid.gridWidth);
  let center = ((u.dropCenter * 0.5) + 0.5);
  let r = max(u.dropRadius, 0.0001);
  let d = max(0.0, (1.0 - (length((center - uv)) / r)));
  let bump = (0.5 - (cos((d * 3.141592653589793)) * 0.5));
  let h = (here.x + (bump * u.dropStrength));
  outState[cellIndex] = vec4<f32>(h, here.y, here.z, here.w);
}
