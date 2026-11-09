struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
  shadowColor : vec3<f32>,
  outlineColor : vec3<f32>,
  bands : f32,
  outlineWidth : f32,
  cameraPos : vec3<f32>,
  sunDir : vec3<f32>,
  sunColor : vec3<f32>,
  ambient : vec3<f32>,
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
  let l = normalize(u.sunDir);
  let selenaLib0 = dot(n, l);
  let selenaLib1 = u.bands;
  let selenaLib2 = 0.08;
  let selenaLib3 = max(selenaLib1, 1.0);
  let selenaLib4 = (clamp(selenaLib0, 0.0, 1.0) * selenaLib3);
  let selenaLib5 = ((floor(selenaLib4) + smoothstep((1.0 - max(selenaLib2, 0.0001)), 1.0, fract(selenaLib4))) / selenaLib3);
  let light = selenaLib5;
  let body = (mix(u.shadowColor, u.baseColor, light) * (u.sunColor + u.ambient));
  let selenaLib6 = n;
  let selenaLib7 = v;
  let selenaLib8 = l;
  let selenaLib9 = 0.4;
  let selenaLib10 = 0.01;
  let selenaLib11 = (1.0 - (clamp(selenaLib9, 0.0, 1.0) * 0.1));
  let selenaLib12 = smoothstep((selenaLib11 - selenaLib10), (selenaLib11 + selenaLib10), dot(selenaLib6, normalize((selenaLib8 + selenaLib7))));
  let highlight = (selenaLib12 * u.sunColor);
  let selenaLib13 = n;
  let selenaLib14 = v;
  let selenaLib15 = l;
  let selenaLib16 = 0.35;
  let selenaLib17 = 0.04;
  let selenaLib18 = ((1.0 - clamp(dot(selenaLib13, selenaLib14), 0.0, 1.0)) * clamp(((dot(selenaLib13, selenaLib15) * 0.5) + 0.5), 0.0, 1.0));
  let selenaLib19 = (1.0 - clamp(selenaLib16, 0.0, 1.0));
  let selenaLib20 = smoothstep((selenaLib19 - selenaLib17), (selenaLib19 + selenaLib17), selenaLib18);
  let rim = (selenaLib20 * 0.6);
  let shaded = ((body + highlight) + (u.baseColor * rim));
  let selenaLib21 = n;
  let selenaLib22 = v;
  let selenaLib23 = u.outlineWidth;
  let selenaLib24 = dot(selenaLib21, selenaLib22);
  let selenaLib25 = max((fwidth(selenaLib24) * max(selenaLib23, 0.0)), 0.0001);
  let selenaLib26 = (1.0 - smoothstep(selenaLib25, (selenaLib25 * 2.0), selenaLib24));
  return vec4<f32>(mix(shaded, u.outlineColor, selenaLib26), 1.0);
}
