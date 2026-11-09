struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  surfaceColor : vec3<f32>,
  deepColor : vec3<f32>,
  sparkle : f32,
  cameraPos : vec3<f32>,
  ambient : vec3<f32>,
  fog : vec4<f32>,
  lightCount : f32,
  sunDir : vec3<f32>,
  sunColor : vec3<f32>,
  lights : array<vec4<f32>, 32>,
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
  let selenaLib0 = u.surfaceColor;
  let selenaLib1 = u.deepColor;
  let selenaLib2 = n;
  let selenaLib3 = v;
  let selenaLib4 = mix(selenaLib0, selenaLib1, clamp(dot(selenaLib2, selenaLib3), 0.0, 1.0));
  let body = selenaLib4;
  var lit : vec3<f32> = (u.ambient * body);
  for (var i : i32 = 0i; (i < 8i); i = (i + 1i)) {
    let selenaLib5 = i;
    let selenaLib6 = n;
    let selenaLib7 = in.vWorldPos;
    let selenaLib8 = selenaLib5;
    let selenaLib9 = u.lights[(selenaLib8 * 4i)].w;
    let selenaLib10 = selenaLib9;
    let selenaLib11 = ((selenaLib10 == 0.0) || (selenaLib10 == 4.0));
    let selenaLib12 = selenaLib5;
    let selenaLib13 = selenaLib7;
    let selenaLib14 = u.lights[(selenaLib12 * 4i)];
    let selenaLib15 = u.lights[((selenaLib12 * 4i) + 1i)];
    let selenaLib16 = (selenaLib14.xyz - selenaLib13);
    let selenaLib17 = ((selenaLib14.w == 2.0) || (selenaLib14.w == 3.0));
    let selenaLib18 = select(normalize(((-selenaLib15.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib16 / max(length(selenaLib16), 0.0001)), selenaLib17);
    let selenaLib19 = select(max(dot(selenaLib6, selenaLib18), 0.0), 1.0, selenaLib11);
    let selenaLib20 = selenaLib5;
    let selenaLib21 = selenaLib6;
    let selenaLib22 = selenaLib7;
    let selenaLib23 = u.lights[(selenaLib20 * 4i)];
    let selenaLib24 = u.lights[((selenaLib20 * 4i) + 1i)];
    let selenaLib25 = u.lights[((selenaLib20 * 4i) + 2i)];
    let selenaLib26 = u.lights[((selenaLib20 * 4i) + 3i)];
    let selenaLib27 = length((selenaLib23.xyz - selenaLib22));
    let selenaLib28 = (pow(clamp((1.0 - pow((selenaLib27 / max(selenaLib24.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib27 * selenaLib27), 0.0001));
    let selenaLib29 = (1.0 / max(pow(max(selenaLib27, 0.0001), selenaLib25.w), 0.0001));
    let selenaLib30 = select(1.0, select(selenaLib29, selenaLib28, (selenaLib24.w > 0.0)), ((selenaLib23.w == 2.0) || (selenaLib23.w == 3.0)));
    let selenaLib31 = selenaLib20;
    let selenaLib32 = selenaLib22;
    let selenaLib33 = u.lights[(selenaLib31 * 4i)];
    let selenaLib34 = u.lights[((selenaLib31 * 4i) + 1i)];
    let selenaLib35 = (selenaLib33.xyz - selenaLib32);
    let selenaLib36 = ((selenaLib33.w == 2.0) || (selenaLib33.w == 3.0));
    let selenaLib37 = select(normalize(((-selenaLib34.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib35 / max(length(selenaLib35), 0.0001)), selenaLib36);
    let selenaLib38 = dot((-selenaLib37), normalize((selenaLib24.xyz + vec3<f32>(0.0, 0.000001, 0.0))));
    let selenaLib39 = select(1.0, smoothstep(selenaLib26.x, selenaLib26.y, selenaLib38), (selenaLib23.w == 3.0));
    let selenaLib40 = select(selenaLib25.xyz, mix(selenaLib26.xyz, selenaLib25.xyz, clamp(((selenaLib21.y * 0.5) + 0.5), 0.0, 1.0)), (selenaLib23.w == 4.0));
    let selenaLib41 = selenaLib20;
    let selenaLib42 = select(0.0, 1.0, (f32(selenaLib41) < u.lightCount));
    let selenaLib43 = (((selenaLib40 * selenaLib30) * selenaLib39) * selenaLib42);
    let selenaLib44 = (selenaLib43 * selenaLib19);
    let selenaLib45 = i;
    let selenaLib46 = n;
    let selenaLib47 = v;
    let selenaLib48 = in.vWorldPos;
    let selenaLib49 = 96.0;
    let selenaLib50 = selenaLib45;
    let selenaLib51 = u.lights[(selenaLib50 * 4i)].w;
    let selenaLib52 = selenaLib51;
    let selenaLib53 = selenaLib45;
    let selenaLib54 = selenaLib48;
    let selenaLib55 = u.lights[(selenaLib53 * 4i)];
    let selenaLib56 = u.lights[((selenaLib53 * 4i) + 1i)];
    let selenaLib57 = (selenaLib55.xyz - selenaLib54);
    let selenaLib58 = ((selenaLib55.w == 2.0) || (selenaLib55.w == 3.0));
    let selenaLib59 = select(normalize(((-selenaLib56.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib57 / max(length(selenaLib57), 0.0001)), selenaLib58);
    let selenaLib60 = selenaLib59;
    let selenaLib61 = normalize((selenaLib60 + selenaLib47));
    let selenaLib62 = select(0.0, 1.0, (((dot(selenaLib46, selenaLib60) > 0.0) && (selenaLib52 != 0.0)) && (selenaLib52 != 4.0)));
    let selenaLib63 = ((selenaLib49 + 8.0) / 25.1327412287);
    let selenaLib64 = selenaLib45;
    let selenaLib65 = selenaLib46;
    let selenaLib66 = selenaLib48;
    let selenaLib67 = u.lights[(selenaLib64 * 4i)];
    let selenaLib68 = u.lights[((selenaLib64 * 4i) + 1i)];
    let selenaLib69 = u.lights[((selenaLib64 * 4i) + 2i)];
    let selenaLib70 = u.lights[((selenaLib64 * 4i) + 3i)];
    let selenaLib71 = length((selenaLib67.xyz - selenaLib66));
    let selenaLib72 = (pow(clamp((1.0 - pow((selenaLib71 / max(selenaLib68.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib71 * selenaLib71), 0.0001));
    let selenaLib73 = (1.0 / max(pow(max(selenaLib71, 0.0001), selenaLib69.w), 0.0001));
    let selenaLib74 = select(1.0, select(selenaLib73, selenaLib72, (selenaLib68.w > 0.0)), ((selenaLib67.w == 2.0) || (selenaLib67.w == 3.0)));
    let selenaLib75 = selenaLib64;
    let selenaLib76 = selenaLib66;
    let selenaLib77 = u.lights[(selenaLib75 * 4i)];
    let selenaLib78 = u.lights[((selenaLib75 * 4i) + 1i)];
    let selenaLib79 = (selenaLib77.xyz - selenaLib76);
    let selenaLib80 = ((selenaLib77.w == 2.0) || (selenaLib77.w == 3.0));
    let selenaLib81 = select(normalize(((-selenaLib78.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib79 / max(length(selenaLib79), 0.0001)), selenaLib80);
    let selenaLib82 = dot((-selenaLib81), normalize((selenaLib68.xyz + vec3<f32>(0.0, 0.000001, 0.0))));
    let selenaLib83 = select(1.0, smoothstep(selenaLib70.x, selenaLib70.y, selenaLib82), (selenaLib67.w == 3.0));
    let selenaLib84 = select(selenaLib69.xyz, mix(selenaLib70.xyz, selenaLib69.xyz, clamp(((selenaLib65.y * 0.5) + 0.5), 0.0, 1.0)), (selenaLib67.w == 4.0));
    let selenaLib85 = selenaLib64;
    let selenaLib86 = select(0.0, 1.0, (f32(selenaLib85) < u.lightCount));
    let selenaLib87 = (((selenaLib84 * selenaLib74) * selenaLib83) * selenaLib86);
    let selenaLib88 = (((selenaLib87 * pow(max(dot(selenaLib46, selenaLib61), 0.0), selenaLib49)) * selenaLib63) * selenaLib62);
    lit = ((lit + (body * selenaLib44)) + (0.6 * selenaLib88));
  }
  let selenaLib89 = in.vWorldPos;
  let selenaLib90 = n;
  let selenaLib91 = v;
  let selenaLib92 = normalize(u.sunDir);
  let selenaLib93 = u.sparkle;
  let selenaLib94 = floor((selenaLib89 * selenaLib93));
  let selenaLib95 = (selenaLib94.xy + vec2<f32>((selenaLib94.z * 13.1), (selenaLib94.z * 7.7)));
  let selenaLib96 = fract((sin(dot(selenaLib95, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib97 = selenaLib96;
  let selenaLib98 = selenaLib94.xz;
  let selenaLib99 = fract((sin(dot(selenaLib98, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib100 = (selenaLib94.yz + vec2<f32>(3.7, 1.3));
  let selenaLib101 = fract((sin(dot(selenaLib100, vec2<f32>(127.1, 311.7))) * 43758.5453));
  let selenaLib102 = vec3<f32>((selenaLib99 - 0.5), (selenaLib101 - 0.5), (selenaLib97 - 0.5));
  let selenaLib103 = normalize((selenaLib90 + (selenaLib102 * 0.6)));
  let selenaLib104 = (pow(clamp(dot(selenaLib103, normalize((selenaLib92 + selenaLib91))), 0.0, 1.0), 200.0) * step(0.7, selenaLib97));
  let glint = ((selenaLib104 * u.sunColor) * 4.0);
  let selenaLib105 = n;
  let selenaLib106 = v;
  let selenaLib107 = 4.0;
  let selenaLib108 = pow((1.0 - clamp(dot(selenaLib105, selenaLib106), 0.0, 1.0)), selenaLib107);
  let rim = ((selenaLib108 * u.surfaceColor) * 0.5);
  let selenaLib109 = ((lit + glint) + rim);
  let selenaLib110 = in.vWorldPos;
  let selenaLib111 = selenaLib110;
  let selenaLib112 = (length((selenaLib111 - u.cameraPos)) * u.fog.w);
  let selenaLib113 = clamp(exp((-(selenaLib112 * selenaLib112))), 0.0, 1.0);
  let selenaLib114 = mix(u.fog.xyz, selenaLib109, selenaLib113);
  return vec4<f32>(selenaLib114, 1.0);
}
