struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  threshold : f32,
  colorA : vec3<f32>,
  colorB : vec3<f32>,
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
  let su = in.vUv.x;
  let sv = in.vUv.y;
  let band = select(0.0, 1.0, (su > u.threshold));
  let pick = select(0.0, select(0.5, 1.0, (sv > 0.5)), (su > 0.5));
  let pick2 = select(select(1.0, 0.75, (sv > 0.5)), 0.25, (su > 0.5));
  let m = clamp(select(pick, band, (su > u.threshold)), 0.0, 1.0);
  let shade = ((select(pick2, pick, (m > 0.5)) * band) + (1.0 - m));
  return vec4<f32>(((u.colorA * shade) + (u.colorB * (1.0 - shade))), 1.0);
}
