struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  depthFade : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var bump : texture_2d<f32>;
@group(0) @binding(2) var bumpSampler : sampler;
@group(0) @binding(3) var planarReflection : texture_2d<f32>;
@group(0) @binding(4) var planarReflectionSampler : sampler;
@group(0) @binding(5) var opaqueColor : texture_2d<f32>;
@group(0) @binding(6) var opaqueColorSampler : sampler;
@group(0) @binding(7) var opaqueDepth : texture_depth_2d;
@group(0) @binding(8) var opaqueDepthSampler : sampler;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vUv : vec2<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let d = textureLoad(opaqueDepth, vec2<i32>(clamp(floor((in.uv) * vec2<f32>(textureDimensions(opaqueDepth))), vec2<f32>(0.0), vec2<f32>(textureDimensions(opaqueDepth)) - vec2<f32>(1.0))), 0);
  out.vUv = in.uv;
  out.position = (u.mvp * vec4<f32>((in.position + (in.normal * (d * 0.01))), 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let b = textureSample(bump, bumpSampler, in.vUv).r;
  let refl = textureSample(planarReflection, planarReflectionSampler, (in.vUv + vec2<f32>((b * 0.02), 0.0))).rgb;
  let under = textureSample(opaqueColor, opaqueColorSampler, in.vUv).rgb;
  let depth = textureLoad(opaqueDepth, vec2<i32>(clamp(floor((in.vUv) * vec2<f32>(textureDimensions(opaqueDepth))), vec2<f32>(0.0), vec2<f32>(textureDimensions(opaqueDepth)) - vec2<f32>(1.0))), 0);
  let k = clamp((depth * u.depthFade), 0.0, 1.0);
  return vec4<f32>(mix(under, refl, k), 1.0);
}
