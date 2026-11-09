struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  tint : vec3<f32>,
  hitColor : vec3<f32>,
  hitPos : vec3<f32>,
  hitAge : f32,
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
  let selenaLib2 = select(selenaLib0, (-selenaLib0), (dot(selenaLib0, selenaLib1) < 0.0));
  let selenaLib3 = selenaLib2;
  let selenaLib4 = v;
  let selenaLib5 = in.vWorldPos;
  let selenaLib6 = u.time;
  let selenaLib7 = selenaLib3;
  let selenaLib8 = selenaLib4;
  let selenaLib9 = 3.0;
  let selenaLib10 = pow((1.0 - clamp(dot(selenaLib7, selenaLib8), 0.0, 1.0)), selenaLib9);
  let selenaLib11 = selenaLib10;
  let selenaLib12 = (selenaLib5.y + (selenaLib5.x * 0.3));
  let selenaLib13 = selenaLib6;
  let selenaLib14 = 3.0;
  let selenaLib15 = 0.4;
  let selenaLib16 = 4.0;
  let selenaLib17 = fract(((selenaLib12 * selenaLib14) - (selenaLib13 * selenaLib15)));
  let selenaLib18 = pow((1.0 - abs(((selenaLib17 * 2.0) - 1.0))), max(selenaLib16, 1.0));
  let selenaLib19 = (selenaLib18 * 0.35);
  let selenaLib20 = clamp((selenaLib11 + selenaLib19), 0.0, 2.0);
  let shell = selenaLib20;
  let selenaLib21 = in.vWorldPos;
  let selenaLib22 = u.hitPos;
  let selenaLib23 = u.hitAge;
  let selenaLib24 = 3.0;
  let selenaLib25 = 0.25;
  let selenaLib26 = abs((distance(selenaLib21, selenaLib22) - (max(selenaLib23, 0.0) * selenaLib24)));
  let selenaLib27 = select(0.0, clamp((1.0 - selenaLib23), 0.0, 1.0), (selenaLib23 >= 0.0));
  let selenaLib28 = ((1.0 - smoothstep(0.0, max(selenaLib25, 0.0001), selenaLib26)) * selenaLib27);
  let hit = selenaLib28;
  return vec4<f32>(((u.tint * shell) + (u.hitColor * hit)), 1.0);
}
