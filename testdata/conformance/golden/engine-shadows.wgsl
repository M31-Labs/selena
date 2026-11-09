struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  shadow0 : vec4<f32>,
  shadowMatrices0 : array<vec4<f32>, 16>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var shadowMap0 : texture_depth_2d_array;
@group(0) @binding(2) var shadowMap0Sampler : sampler;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vLift : f32,
  @location(1) vPos : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vLift = textureLoad(shadowMap0, vec2<i32>(clamp(floor((in.uv) * vec2<f32>(textureDimensions(shadowMap0))), vec2<f32>(0.0), vec2<f32>(textureDimensions(shadowMap0)) - vec2<f32>(1.0))), 0i, 0);
  out.vPos = in.position;
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let c = (i32(u.shadow0.x) - 1i);
  let s = ((u.shadowMatrices0[(c * 4i)] * in.vPos.x) + u.shadowMatrices0[((c * 4i) + 3i)]);
  let lit = select(0.0, 1.0, ((s.z - u.shadow0.y) <= textureLoad(shadowMap0, vec2<i32>(clamp(floor((s.xy) * vec2<f32>(textureDimensions(shadowMap0))), vec2<f32>(0.0), vec2<f32>(textureDimensions(shadowMap0)) - vec2<f32>(1.0))), c, 0)));
  return vec4<f32>(vec3<f32>(lit, lit, in.vLift), 1.0);
}
