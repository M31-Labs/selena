struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) rect : vec4<f32>,
  @location(1) tone : vec4<f32>,
  @location(2) spin : f32,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vTone : vec4<f32>,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex : u32, in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let fi = f32(vertexIndex);
  let corner = vec2<f32>((fract((fi * 0.5)) * 2.0), floor((fi * 0.5)));
  let c = cos(in.spin);
  let s = sin(in.spin);
  let centered = (corner - vec2<f32>(0.5, 0.5));
  let turned = vec2<f32>(((centered.x * c) - (centered.y * s)), ((centered.x * s) + (centered.y * c)));
  let p = (in.rect.xy + ((turned + vec2<f32>(0.5, 0.5)) * in.rect.zw));
  out.vTone = in.tone;
  out.position = (u.mvp * vec4<f32>(p, 0.0, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  return vec4<f32>(vec3<f32>(in.vTone.r, in.vTone.g, in.vTone.b), 1.0);
}
