struct Uniforms {
  head : f32,
  gain : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var spectrum : texture_2d<f32>;
@group(0) @binding(2) var spectrumSampler : sampler;
@group(0) @binding(3) var palette : texture_2d<f32>;
@group(0) @binding(4) var paletteSampler : sampler;

struct ScreenOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) v_uv           : vec2<f32>,
};

const _screenPositions = array<vec2<f32>, 3>(
  vec2<f32>(-1.0, -1.0),
  vec2<f32>( 3.0, -1.0),
  vec2<f32>(-1.0,  3.0),
);
const _screenUVs = array<vec2<f32>, 3>(
  vec2<f32>(0.0, 1.0),
  vec2<f32>(2.0, 1.0),
  vec2<f32>(0.0, -1.0),
);

@vertex fn vertexMain(@builtin(vertex_index) vi : u32) -> ScreenOutput {
  var out : ScreenOutput;
  out.position = vec4<f32>(_screenPositions[vi], 0.0, 1.0);
  out.v_uv     = _screenUVs[vi];
  return out;
}

struct ScreenInput {
  @location(0) v_uv : vec2<f32>,
};

@fragment fn fragmentMain(in : ScreenInput) -> @location(0) vec4<f32> {
  let sz = vec2<f32>(textureDimensions(spectrum));
  let col = i32((((in.v_uv.x * sz.x) + u.head) - (sz.x * floor(((in.v_uv.x * sz.x) + u.head) / sz.x))));
  let row = i32(min((in.v_uv.y * sz.y), (sz.y - 1.0)));
  let mag = clamp((textureLoad(spectrum, vec2<i32>(col, row), 0).r * u.gain), 0.0, 1.0);
  let c = textureSample(palette, paletteSampler, vec2<f32>(mag, 0.5));
  return vec4<f32>((c.r * mag), (c.g * mag), (c.b * mag), mag);
}
