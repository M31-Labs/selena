struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

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
  out.vUv = in.uv;
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let parity = false;
  let toggled = (parity != true);
  let same = ((in.vUv.x < 0.5) == toggled);
  let visible = ((!parity) && (same || false));
  return vec4<f32>(select(vec3<f32>(0.1, 0.2, 0.4), vec3<f32>(0.9, 0.7, 0.2), visible), 1.0);
}
