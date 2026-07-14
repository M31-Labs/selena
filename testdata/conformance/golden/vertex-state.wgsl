struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct StateGrid {
  gridWidth  : u32,
  gridHeight : u32,
};
@group(0) @binding(1) var<uniform> _stateGrid : StateGrid;
@group(0) @binding(2) var _inState : texture_2d<f32>;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vUv : vec2<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let s = textureLoad(_inState, vec2<u32>(min(u32((in.uv).x * f32(_stateGrid.gridWidth)), _stateGrid.gridWidth - 1u), min(u32((in.uv).y * f32(_stateGrid.gridHeight)), _stateGrid.gridHeight - 1u)), 0);
  let y = s.x;
  out.vUv = in.uv;
  out.position = (u.mvp * vec4<f32>(in.position.x, y, in.position.z, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let s = textureLoad(_inState, vec2<u32>(min(u32((in.vUv).x * f32(_stateGrid.gridWidth)), _stateGrid.gridWidth - 1u), min(u32((in.vUv).y * f32(_stateGrid.gridHeight)), _stateGrid.gridHeight - 1u)), 0);
  return vec4<f32>(vec3<f32>(((s.x * 0.5) + 0.5), ((s.y * 0.5) + 0.5), ((s.z * 0.5) + 0.5)), 1.0);
}
