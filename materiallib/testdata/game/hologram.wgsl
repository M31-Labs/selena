struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  tint : vec3<f32>,
  density : f32,
  flicker : f32,
  cameraPos : vec3<f32>,
  time : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vWorldNormal : vec3<f32>,
  @location(1) vWorldPos : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.vWorldPos = in.position;
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let n = normalize(in.vWorldNormal);
  let v = normalize((u.cameraPos - in.vWorldPos));
  let selenaLib0 = n;
  let selenaLib1 = v;
  let selenaLib2 = in.vWorldPos;
  let selenaLib3 = u.time;
  let selenaLib4 = u.density;
  let selenaLib5 = u.flicker;
  let selenaLib6 = selenaLib0;
  let selenaLib7 = selenaLib1;
  let selenaLib8 = 2.0;
  let selenaLib9 = pow((1.0 - clamp(dot(selenaLib6, selenaLib7), 0.0, 1.0)), selenaLib8);
  let selenaLib10 = selenaLib9;
  let selenaLib11 = selenaLib2.y;
  let selenaLib12 = selenaLib3;
  let selenaLib13 = selenaLib4;
  let selenaLib14 = 0.6;
  let selenaLib15 = 8.0;
  let selenaLib16 = fract(((selenaLib11 * selenaLib13) - (selenaLib12 * selenaLib14)));
  let selenaLib17 = pow((1.0 - abs(((selenaLib16 * 2.0) - 1.0))), max(selenaLib15, 1.0));
  let selenaLib18 = selenaLib17;
  let selenaLib19 = vec2<f32>(floor((selenaLib3 * 24.0)), 7.0);
  let selenaLib20 = fract((sin(dot(selenaLib19, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib21 = (1.0 - (clamp(selenaLib5, 0.0, 1.0) * selenaLib20));
  let selenaLib22 = (((0.25 + (selenaLib10 * 0.9)) + (selenaLib18 * 0.5)) * selenaLib21);
  return vec4<f32>((u.tint * selenaLib22), 1.0);
}
