struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  labelRect : vec4<f32>,
  numberRect : vec4<f32>,
  shirtColor : vec3<f32>,
  numberColor : vec3<f32>,
  sunDir : vec3<f32>,
  sunColor : vec3<f32>,
  ambient : vec3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var labels : texture_2d<f32>;
@group(0) @binding(2) var labelsSampler : sampler;

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
  let selenaLib0 = in.vUv;
  let selenaLib1 = u.numberRect;
  let selenaLib2 = ((selenaLib0 - selenaLib1.xy) / max((selenaLib1.zw - selenaLib1.xy), vec2<f32>(0.000001, 0.000001)));
  let selenaLib3 = selenaLib2;
  let selenaLib4 = u.labelRect;
  let selenaLib5 = mix(selenaLib4.xy, selenaLib4.zw, clamp(selenaLib3, vec2<f32>(0.0, 0.0), vec2<f32>(1.0, 1.0)));
  let glyph = textureSample(labels, labelsSampler, selenaLib5);
  let selenaLib6 = in.vUv;
  let selenaLib7 = u.numberRect;
  let selenaLib8 = selenaLib6;
  let selenaLib9 = selenaLib7;
  let selenaLib10 = ((selenaLib8 - selenaLib9.xy) / max((selenaLib9.zw - selenaLib9.xy), vec2<f32>(0.000001, 0.000001)));
  let selenaLib11 = selenaLib10;
  let selenaLib12 = (((step(0.0, selenaLib11.x) * step(selenaLib11.x, 1.0)) * step(0.0, selenaLib11.y)) * step(selenaLib11.y, 1.0));
  let ink = (glyph.a * selenaLib12);
  let n = normalize(in.vWorldNormal);
  let light = (u.ambient + (u.sunColor * max(dot(n, normalize(u.sunDir)), 0.0)));
  return vec4<f32>((mix(u.shirtColor, (u.numberColor * glyph.rgb), ink) * light), 1.0);
}
