struct GridUniforms {
  gridWidth : u32,
  gridLen   : u32,
};
@group(0) @binding(0) var<uniform> _grid : GridUniforms;
@group(0) @binding(1) var<storage, read> inState : array<vec4<f32>>;
@group(0) @binding(2) var<storage, read_write> outState : array<vec4<f32>>;

struct UserUniforms {
  dropCount : f32,
  dropRadius : f32,
  drops : array<vec4<f32>, 64>,
};
@group(0) @binding(3) var<uniform> u : UserUniforms;

@compute @workgroup_size(64)
fn computeMain(@builtin(global_invocation_id) gid : vec3<u32>) {
  let cellIndex = gid.x;
  if (cellIndex >= _grid.gridLen) { return; }
  let here = inState[clamp(i32(cellIndex) + (0) + (0) * i32(_grid.gridWidth), 0, i32(_grid.gridLen) - 1)];
  let uv = (vec2<f32>(f32(cellIndex % _grid.gridWidth), f32(cellIndex / _grid.gridWidth)) + vec2<f32>(0.5, 0.5)) / f32(_grid.gridWidth);
  var h : f32 = here.x;
  let count = i32(u.dropCount);
  for (var j : i32 = 0i; (j < count); j = (j + 1i)) {
    let drop = u.drops[j];
    let center = drop.xy;
    let r = max(u.dropRadius, 0.0001);
    let dist = max(0.0, (1.0 - (length((center - uv)) / r)));
    let bump = (0.5 - (cos((dist * 3.141592653589793)) * 0.5));
    h = (h + (bump * drop.w));
  }
  outState[cellIndex] = vec4<f32>(h, here.y, here.z, here.w);
}
