struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  deepColor : vec3<f32>,
  skyColor : vec3<f32>,
  waveScale : f32,
  choppy : f32,
  cameraPos : vec3<f32>,
  sunDir : vec3<f32>,
  sunColor : vec3<f32>,
  time : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vWorldPos : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vWorldPos = in.position;
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let selenaLib0 = in.vWorldPos.xz;
  let selenaLib1 = u.time;
  let selenaLib2 = u.waveScale;
  let selenaLib3 = u.choppy;
  let selenaLib4 = (selenaLib0 * selenaLib2);
  let selenaLib5 = (vec2<f32>(0.8, 0.6) * cos((dot(selenaLib4, vec2<f32>(0.8, 0.6)) + (selenaLib1 * 1.3))));
  let selenaLib6 = (vec2<f32>((-0.4), 0.9) * (cos(((dot(selenaLib4, vec2<f32>((-0.4), 0.9)) * 1.7) + (selenaLib1 * 1.9))) * 0.6));
  let selenaLib7 = (vec2<f32>(0.95, (-0.3)) * (cos(((dot(selenaLib4, vec2<f32>(0.95, (-0.3))) * 2.9) + (selenaLib1 * 2.7))) * 0.35));
  let selenaLib8 = (((selenaLib5 + selenaLib6) + selenaLib7) * selenaLib3);
  let selenaLib9 = normalize(vec3<f32>((-selenaLib8.x), 1.0, (-selenaLib8.y)));
  let n = selenaLib9;
  let v = normalize((u.cameraPos - in.vWorldPos));
  let l = normalize(u.sunDir);
  let selenaLib10 = n;
  let selenaLib11 = v;
  let selenaLib12 = (1.0 - clamp(dot(selenaLib10, selenaLib11), 0.0, 1.0));
  let selenaLib13 = (selenaLib12 * selenaLib12);
  let selenaLib14 = (0.02 + (((0.98 * selenaLib13) * selenaLib13) * selenaLib12));
  let f = selenaLib14;
  let glint = ((pow(max(dot(reflect((-l), n), v), 0.0), 240.0) * u.sunColor) * 6.0);
  return vec4<f32>((mix(u.deepColor, u.skyColor, f) + glint), 1.0);
}
