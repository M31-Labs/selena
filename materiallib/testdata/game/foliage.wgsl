struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  leafColor : vec3<f32>,
  backColor : vec3<f32>,
  stiffness : f32,
  thickness : f32,
  cameraPos : vec3<f32>,
  ambient : vec3<f32>,
  fog : vec4<f32>,
  lightCount : f32,
  wind : vec4<f32>,
  windGust : vec4<f32>,
  lights : array<vec4<f32>, 32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) worldPos : vec3<f32>,
  @location(1) worldNormal : vec3<f32>,
  @location(2) leafUV : vec2<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  let selenaLib95 = in.position;
  let selenaLib96 = in.uv.y;
  let selenaLib97 = u.stiffness;
  let selenaLib98 = u.wind;
  let selenaLib99 = u.windGust;
  let selenaLib100 = selenaLib95;
  let selenaLib101 = selenaLib98;
  let selenaLib102 = selenaLib99;
  let selenaLib103 = max(selenaLib102.y, 1.0);
  let selenaLib104 = ((selenaLib100.x * selenaLib101.x) + (selenaLib100.z * selenaLib101.y));
  let selenaLib105 = ((selenaLib100.x * selenaLib101.y) - (selenaLib100.z * selenaLib101.x));
  let selenaLib106 = (0.35 * sin(((selenaLib105 * 6.2831855) / (selenaLib103 * 1.7))));
  let selenaLib107 = ((((selenaLib104 - (selenaLib102.z * selenaLib101.w)) * 6.2831855) / selenaLib103) + selenaLib106);
  let selenaLib108 = (1.0 + (selenaLib102.x * sin(selenaLib107)));
  let selenaLib109 = ((clamp((selenaLib98.z / 10.0), 0.0, 1.5) * selenaLib108) / max(selenaLib97, 0.05));
  let selenaLib110 = (sin((((selenaLib98.w * selenaLib99.w) * 6.2831855) + dot(selenaLib95.xz, vec2<f32>(0.37, 0.61)))) * 0.25);
  let selenaLib111 = (((selenaLib96 * selenaLib96) * selenaLib109) * (1.0 + selenaLib110));
  let selenaLib112 = vec3<f32>(((selenaLib98.x * selenaLib111) * 0.3), ((-(selenaLib111 * selenaLib111)) * 0.03), ((selenaLib98.y * selenaLib111) * 0.3));
  let p = (in.position + selenaLib112);
  out.worldPos = p;
  out.worldNormal = (u.normalMatrix * in.normal);
  out.leafUV = in.uv;
  out.position = (u.mvp * vec4<f32>(p, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let v = normalize((u.cameraPos - in.worldPos));
  let selenaLib0 = normalize(in.worldNormal);
  let selenaLib1 = v;
  let selenaLib2 = select(selenaLib0, (-selenaLib0), (dot(selenaLib0, selenaLib1) < 0.0));
  let n = selenaLib2;
  var lit : vec3<f32> = ((u.ambient * u.leafColor) * (0.6 + (0.4 * in.leafUV.y)));
  for (var i : i32 = 0i; (i < 8i); i = (i + 1i)) {
    let selenaLib3 = i;
    let selenaLib4 = in.worldPos;
    let selenaLib5 = u.lights[(selenaLib3 * 4i)];
    let selenaLib6 = u.lights[((selenaLib3 * 4i) + 1i)];
    let selenaLib7 = (selenaLib5.xyz - selenaLib4);
    let selenaLib8 = ((selenaLib5.w == 2.0) || (selenaLib5.w == 3.0));
    let selenaLib9 = select(normalize(((-selenaLib6.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib7 / max(length(selenaLib7), 0.0001)), selenaLib8);
    let l = selenaLib9;
    let selenaLib10 = n;
    let selenaLib11 = v;
    let selenaLib12 = l;
    let selenaLib13 = u.thickness;
    let selenaLib14 = selenaLib10;
    let selenaLib15 = selenaLib11;
    let selenaLib16 = selenaLib12;
    let selenaLib17 = 0.35;
    let selenaLib18 = 4.0;
    let selenaLib19 = selenaLib13;
    let selenaLib20 = normalize((selenaLib16 + (selenaLib14 * selenaLib17)));
    let selenaLib21 = (pow(clamp(dot(selenaLib15, (-selenaLib20)), 0.0, 1.0), max(selenaLib18, 1.0)) * (1.0 - clamp(selenaLib19, 0.0, 1.0)));
    let selenaLib22 = selenaLib21;
    let selenaLib23 = i;
    let selenaLib24 = select(0.0, 1.0, (f32(selenaLib23) < u.lightCount));
    let back = (selenaLib22 * selenaLib24);
    let selenaLib25 = i;
    let selenaLib26 = n;
    let selenaLib27 = in.worldPos;
    let selenaLib28 = selenaLib25;
    let selenaLib29 = u.lights[(selenaLib28 * 4i)].w;
    let selenaLib30 = selenaLib29;
    let selenaLib31 = ((selenaLib30 == 0.0) || (selenaLib30 == 4.0));
    let selenaLib32 = selenaLib25;
    let selenaLib33 = selenaLib27;
    let selenaLib34 = u.lights[(selenaLib32 * 4i)];
    let selenaLib35 = u.lights[((selenaLib32 * 4i) + 1i)];
    let selenaLib36 = (selenaLib34.xyz - selenaLib33);
    let selenaLib37 = ((selenaLib34.w == 2.0) || (selenaLib34.w == 3.0));
    let selenaLib38 = select(normalize(((-selenaLib35.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib36 / max(length(selenaLib36), 0.0001)), selenaLib37);
    let selenaLib39 = select(max(dot(selenaLib26, selenaLib38), 0.0), 1.0, selenaLib31);
    let selenaLib40 = selenaLib25;
    let selenaLib41 = selenaLib26;
    let selenaLib42 = selenaLib27;
    let selenaLib43 = u.lights[(selenaLib40 * 4i)];
    let selenaLib44 = u.lights[((selenaLib40 * 4i) + 1i)];
    let selenaLib45 = u.lights[((selenaLib40 * 4i) + 2i)];
    let selenaLib46 = u.lights[((selenaLib40 * 4i) + 3i)];
    let selenaLib47 = length((selenaLib43.xyz - selenaLib42));
    let selenaLib48 = (pow(clamp((1.0 - pow((selenaLib47 / max(selenaLib44.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib47 * selenaLib47), 0.0001));
    let selenaLib49 = (1.0 / max(pow(max(selenaLib47, 0.0001), selenaLib45.w), 0.0001));
    let selenaLib50 = select(1.0, select(selenaLib49, selenaLib48, (selenaLib44.w > 0.0)), ((selenaLib43.w == 2.0) || (selenaLib43.w == 3.0)));
    let selenaLib51 = selenaLib40;
    let selenaLib52 = selenaLib42;
    let selenaLib53 = u.lights[(selenaLib51 * 4i)];
    let selenaLib54 = u.lights[((selenaLib51 * 4i) + 1i)];
    let selenaLib55 = (selenaLib53.xyz - selenaLib52);
    let selenaLib56 = ((selenaLib53.w == 2.0) || (selenaLib53.w == 3.0));
    let selenaLib57 = select(normalize(((-selenaLib54.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib55 / max(length(selenaLib55), 0.0001)), selenaLib56);
    let selenaLib58 = dot((-selenaLib57), normalize((selenaLib44.xyz + vec3<f32>(0.0, 0.000001, 0.0))));
    let selenaLib59 = select(1.0, smoothstep(selenaLib46.x, selenaLib46.y, selenaLib58), (selenaLib43.w == 3.0));
    let selenaLib60 = select(selenaLib45.xyz, mix(selenaLib46.xyz, selenaLib45.xyz, clamp(((selenaLib41.y * 0.5) + 0.5), 0.0, 1.0)), (selenaLib43.w == 4.0));
    let selenaLib61 = selenaLib40;
    let selenaLib62 = select(0.0, 1.0, (f32(selenaLib61) < u.lightCount));
    let selenaLib63 = (((selenaLib60 * selenaLib50) * selenaLib59) * selenaLib62);
    let selenaLib64 = (selenaLib63 * selenaLib39);
    let selenaLib65 = i;
    let selenaLib66 = n;
    let selenaLib67 = in.worldPos;
    let selenaLib68 = u.lights[(selenaLib65 * 4i)];
    let selenaLib69 = u.lights[((selenaLib65 * 4i) + 1i)];
    let selenaLib70 = u.lights[((selenaLib65 * 4i) + 2i)];
    let selenaLib71 = u.lights[((selenaLib65 * 4i) + 3i)];
    let selenaLib72 = length((selenaLib68.xyz - selenaLib67));
    let selenaLib73 = (pow(clamp((1.0 - pow((selenaLib72 / max(selenaLib69.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib72 * selenaLib72), 0.0001));
    let selenaLib74 = (1.0 / max(pow(max(selenaLib72, 0.0001), selenaLib70.w), 0.0001));
    let selenaLib75 = select(1.0, select(selenaLib74, selenaLib73, (selenaLib69.w > 0.0)), ((selenaLib68.w == 2.0) || (selenaLib68.w == 3.0)));
    let selenaLib76 = selenaLib65;
    let selenaLib77 = selenaLib67;
    let selenaLib78 = u.lights[(selenaLib76 * 4i)];
    let selenaLib79 = u.lights[((selenaLib76 * 4i) + 1i)];
    let selenaLib80 = (selenaLib78.xyz - selenaLib77);
    let selenaLib81 = ((selenaLib78.w == 2.0) || (selenaLib78.w == 3.0));
    let selenaLib82 = select(normalize(((-selenaLib79.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib80 / max(length(selenaLib80), 0.0001)), selenaLib81);
    let selenaLib83 = dot((-selenaLib82), normalize((selenaLib69.xyz + vec3<f32>(0.0, 0.000001, 0.0))));
    let selenaLib84 = select(1.0, smoothstep(selenaLib71.x, selenaLib71.y, selenaLib83), (selenaLib68.w == 3.0));
    let selenaLib85 = select(selenaLib70.xyz, mix(selenaLib71.xyz, selenaLib70.xyz, clamp(((selenaLib66.y * 0.5) + 0.5), 0.0, 1.0)), (selenaLib68.w == 4.0));
    let selenaLib86 = selenaLib65;
    let selenaLib87 = select(0.0, 1.0, (f32(selenaLib86) < u.lightCount));
    let selenaLib88 = (((selenaLib85 * selenaLib75) * selenaLib84) * selenaLib87);
    lit = ((lit + (u.leafColor * selenaLib64)) + ((u.backColor * selenaLib88) * back));
  }
  let selenaLib89 = lit;
  let selenaLib90 = in.worldPos;
  let selenaLib91 = selenaLib90;
  let selenaLib92 = (length((selenaLib91 - u.cameraPos)) * u.fog.w);
  let selenaLib93 = clamp(exp((-(selenaLib92 * selenaLib92))), 0.0, 1.0);
  let selenaLib94 = mix(u.fog.xyz, selenaLib89, selenaLib93);
  return vec4<f32>(selenaLib94, 1.0);
}
