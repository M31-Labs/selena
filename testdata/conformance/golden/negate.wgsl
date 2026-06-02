struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
  light_ambient : f32,
  light_dir : vec3<f32>,
  cutoff : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vWorldNormal : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let n = normalize(in.vWorldNormal);
  let lambert = max(dot(n, (-u.light_dir)), (-u.cutoff));
  return vec4<f32>((u.baseColor * (u.light_ambient + lambert)), 1.0);
}
