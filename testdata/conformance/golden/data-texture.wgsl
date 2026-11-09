struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  head : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var spectrum : texture_2d<f32>;
@group(0) @binding(2) var spectrumSampler : sampler;
@group(0) @binding(3) var gains : texture_2d<f32>;
@group(0) @binding(4) var gainsSampler : sampler;
@group(0) @binding(5) var palette : texture_2d<f32>;
@group(0) @binding(6) var paletteSampler : sampler;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vLevel : f32,
  @location(1) vUv : vec2<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let columns = vec2<f32>(textureDimensions(gains)).x;
  let column = i32(clamp((in.uv.x * columns), 0.0, (columns - 1.0)));
  let level = textureLoad(gains, vec2<i32>(column, 0i), 0).r;
  out.vLevel = level;
  out.vUv = in.uv;
  out.position = (u.mvp * vec4<f32>((in.position + (in.normal * level)), 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let sz = vec2<f32>(textureDimensions(spectrum));
  let col = i32((((in.vUv.x * sz.x) + u.head) - (sz.x * floor(((in.vUv.x * sz.x) + u.head) / sz.x))));
  let row = i32((in.vUv.y * sz.y));
  let mag = textureLoad(spectrum, vec2<i32>(col, row), 0).r;
  let exact = textureLoad(palette, vec2<i32>(0i, 0i), 0);
  let soft = textureSample(palette, paletteSampler, in.vUv);
  return vec4<f32>(vec3<f32>(((mag * exact.r) + (soft.r * 0.25)), ((mag * 0.6) + in.vLevel), (1.0 - mag)), 1.0);
}
