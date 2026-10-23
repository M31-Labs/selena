struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  scale : f32,
  lod : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var heightMap : texture_2d<f32>;
@group(0) @binding(2) var heightMapSampler : sampler;
@group(0) @binding(3) var tint : texture_2d<f32>;
@group(0) @binding(4) var tintSampler : sampler;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) height : f32,
  @location(1) vUv : vec2<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let h = textureSampleLevel(heightMap, heightMapSampler, in.uv, u.lod).x;
  let p = (in.position + (in.normal * (h * u.scale)));
  out.height = h;
  out.vUv = in.uv;
  out.position = (u.mvp * vec4<f32>(p, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let c = textureSample(tint, tintSampler, in.vUv);
  return vec4<f32>(vec3<f32>((c.x * (0.5 + in.height)), (c.y * (0.5 + in.height)), (c.z * (0.5 + in.height))), 1.0);
}
