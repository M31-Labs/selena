struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  cameraPos : vec3<f32>,
  sunDir : vec3<f32>,
  sunColor : vec3<f32>,
  ambient : vec3<f32>,
  fog : vec4<f32>,
  exposure : f32,
  tint : vec3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vPosition : vec3<f32>,
  @location(1) vWorldNormal : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vPosition = in.position;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let n = normalize(in.vWorldNormal);
  let v = normalize((u.cameraPos - in.vPosition));
  let diffuse = max(dot(n, u.sunDir), 0.0);
  let lit = (((u.ambient + (u.sunColor * diffuse)) * u.tint) * u.exposure);
  let dist = length((u.cameraPos - in.vPosition));
  let f = (1.0 - exp(((-u.fog.w) * dist)));
  return vec4<f32>((mix(lit, u.fog.xyz, f) * (0.5 + (0.5 * dot(n, v)))), 1.0);
}
