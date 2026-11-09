struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
  edgeColor : vec3<f32>,
  progress : f32,
  noiseScale : f32,
  edgeWidth : f32,
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
  let selenaLib0 = (in.vWorldPos * u.noiseScale);
  let selenaLib1 = selenaLib0.xy;
  let selenaLib2 = floor(selenaLib1);
  let selenaLib3 = fract(selenaLib1);
  let selenaLib4 = ((selenaLib3 * selenaLib3) * (3.0 - (2.0 * selenaLib3)));
  let selenaLib5 = selenaLib2;
  let selenaLib6 = fract((sin(dot(selenaLib5, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib7 = selenaLib6;
  let selenaLib8 = (selenaLib2 + vec2<f32>(1.0, 0.0));
  let selenaLib9 = fract((sin(dot(selenaLib8, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib10 = selenaLib9;
  let selenaLib11 = (selenaLib2 + vec2<f32>(0.0, 1.0));
  let selenaLib12 = fract((sin(dot(selenaLib11, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib13 = selenaLib12;
  let selenaLib14 = (selenaLib2 + vec2<f32>(1.0, 1.0));
  let selenaLib15 = fract((sin(dot(selenaLib14, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib16 = selenaLib15;
  let selenaLib17 = mix(mix(selenaLib7, selenaLib10, selenaLib4.x), mix(selenaLib13, selenaLib16, selenaLib4.x), selenaLib4.y);
  let selenaLib18 = (selenaLib0.zx + vec2<f32>(17.0, 31.0));
  let selenaLib19 = floor(selenaLib18);
  let selenaLib20 = fract(selenaLib18);
  let selenaLib21 = ((selenaLib20 * selenaLib20) * (3.0 - (2.0 * selenaLib20)));
  let selenaLib22 = selenaLib19;
  let selenaLib23 = fract((sin(dot(selenaLib22, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib24 = selenaLib23;
  let selenaLib25 = (selenaLib19 + vec2<f32>(1.0, 0.0));
  let selenaLib26 = fract((sin(dot(selenaLib25, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib27 = selenaLib26;
  let selenaLib28 = (selenaLib19 + vec2<f32>(0.0, 1.0));
  let selenaLib29 = fract((sin(dot(selenaLib28, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib30 = selenaLib29;
  let selenaLib31 = (selenaLib19 + vec2<f32>(1.0, 1.0));
  let selenaLib32 = fract((sin(dot(selenaLib31, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib33 = selenaLib32;
  let selenaLib34 = mix(mix(selenaLib24, selenaLib27, selenaLib21.x), mix(selenaLib30, selenaLib33, selenaLib21.x), selenaLib21.y);
  let selenaLib35 = ((selenaLib17 + selenaLib34) * 0.5);
  let selenaLib36 = selenaLib35;
  let selenaLib37 = u.progress;
  let selenaLib38 = ((clamp(selenaLib36, 0.0, 1.0) - (clamp(selenaLib37, 0.0, 1.0) * 1.02)) + 0.01);
  let mask = selenaLib38;
  if ((mask < 0.0)) {
    discard;
  }
  let n = normalize(in.vWorldNormal);
  let lit = (u.baseColor * (u.ambient + (u.sunColor * max(dot(n, normalize(u.sunDir)), 0.0))));
  let selenaLib39 = mask;
  let selenaLib40 = u.edgeWidth;
  let selenaLib41 = (1.0 - smoothstep(0.0, max(selenaLib40, 0.0001), selenaLib39));
  return vec4<f32>((lit + ((u.edgeColor * selenaLib41) * select(0.0, 1.0, (u.progress > 0.0)))), 1.0);
}
