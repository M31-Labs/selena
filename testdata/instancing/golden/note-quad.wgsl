struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) rect : vec4<f32>,
  @location(1) tone : vec4<f32>,
  @location(2) region : i32,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vTone : vec4<f32>,
  @location(1) @interpolate(flat) vRegion : i32,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex : u32, @builtin(instance_index) instanceIndex : u32, in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let corner = vec2<f32>(f32((vertexIndex & 1u)), f32((vertexIndex >> 1u)));
  let p = (in.rect.xy + (corner * in.rect.zw));
  let stripe = (1.0 - (0.04 * f32((instanceIndex & 3u))));
  out.vTone = vec4<f32>((in.tone.rgb * stripe), in.tone.a);
  out.vRegion = in.region;
  out.position = (u.mvp * vec4<f32>(p, 0.0, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let column = f32((in.vRegion & 3i));
  let lift = (0.25 * column);
  return vec4<f32>(vec3<f32>((in.vTone.r + lift), in.vTone.g, in.vTone.b), 1.0);
}
