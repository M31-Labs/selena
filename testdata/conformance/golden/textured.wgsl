struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  light_ambient : f32,
  light_dir : vec3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var albedo : texture_2d<f32>;
@group(0) @binding(2) var albedoSampler : sampler;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vUv : vec2<f32>,
  @location(1) vWorldNormal : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vUv = in.uv;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let c = textureSample(albedo, albedoSampler, in.vUv).rgb;
  return vec4<f32>((c * (u.light_ambient + max(dot(normalize(in.vWorldNormal), u.light_dir), 0.0))), 1.0);
}
