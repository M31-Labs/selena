struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  albedo : vec3<f32>,
  scatter : vec3<f32>,
  wrap : f32,
  thickness : f32,
  cameraPos : vec3<f32>,
  ambient : vec3<f32>,
  fog : vec4<f32>,
  lightCount : f32,
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
  var lit : vec3<f32> = (u.ambient * u.albedo);
  for (var i : i32 = 0i; (i < 8i); i = (i + 1i)) {
    let selenaLib0 = i;
    let selenaLib1 = in.vWorldPos;
    let selenaLib2 = u.lights[(selenaLib0 * 4i)];
    let selenaLib3 = u.lights[((selenaLib0 * 4i) + 1i)];
    let selenaLib4 = (selenaLib2.xyz - selenaLib1);
    let selenaLib5 = ((selenaLib2.w == 2.0) || (selenaLib2.w == 3.0));
    let selenaLib6 = select(normalize(((-selenaLib3.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib4 / max(length(selenaLib4), 0.0001)), selenaLib5);
    let l = selenaLib6;
    let selenaLib7 = i;
    let selenaLib8 = n;
    let selenaLib9 = in.vWorldPos;
    let selenaLib10 = u.lights[(selenaLib7 * 4i)];
    let selenaLib11 = u.lights[((selenaLib7 * 4i) + 1i)];
    let selenaLib12 = u.lights[((selenaLib7 * 4i) + 2i)];
    let selenaLib13 = u.lights[((selenaLib7 * 4i) + 3i)];
    let selenaLib14 = length((selenaLib10.xyz - selenaLib9));
    let selenaLib15 = (pow(clamp((1.0 - pow((selenaLib14 / max(selenaLib11.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib14 * selenaLib14), 0.0001));
    let selenaLib16 = (1.0 / max(pow(max(selenaLib14, 0.0001), selenaLib12.w), 0.0001));
    let selenaLib17 = select(1.0, select(selenaLib16, selenaLib15, (selenaLib11.w > 0.0)), ((selenaLib10.w == 2.0) || (selenaLib10.w == 3.0)));
    let selenaLib18 = selenaLib7;
    let selenaLib19 = selenaLib9;
    let selenaLib20 = u.lights[(selenaLib18 * 4i)];
    let selenaLib21 = u.lights[((selenaLib18 * 4i) + 1i)];
    let selenaLib22 = (selenaLib20.xyz - selenaLib19);
    let selenaLib23 = ((selenaLib20.w == 2.0) || (selenaLib20.w == 3.0));
    let selenaLib24 = select(normalize(((-selenaLib21.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib22 / max(length(selenaLib22), 0.0001)), selenaLib23);
    let selenaLib25 = dot((-selenaLib24), normalize((selenaLib11.xyz + vec3<f32>(0.0, 0.000001, 0.0))));
    let selenaLib26 = select(1.0, smoothstep(selenaLib13.x, selenaLib13.y, selenaLib25), (selenaLib10.w == 3.0));
    let selenaLib27 = select(selenaLib12.xyz, mix(selenaLib13.xyz, selenaLib12.xyz, clamp(((selenaLib8.y * 0.5) + 0.5), 0.0, 1.0)), (selenaLib10.w == 4.0));
    let selenaLib28 = selenaLib7;
    let selenaLib29 = select(0.0, 1.0, (f32(selenaLib28) < u.lightCount));
    let selenaLib30 = (((selenaLib27 * selenaLib17) * selenaLib26) * selenaLib29);
    let radiance = selenaLib30;
    let selenaLib31 = n;
    let selenaLib32 = l;
    let selenaLib33 = u.wrap;
    let selenaLib34 = clamp(((dot(selenaLib31, selenaLib32) + selenaLib33) / (1.0 + selenaLib33)), 0.0, 1.0);
    let selenaLib35 = i;
    let selenaLib36 = select(0.0, 1.0, (f32(selenaLib35) < u.lightCount));
    let soft = (selenaLib34 * selenaLib36);
    let selenaLib37 = n;
    let selenaLib38 = v;
    let selenaLib39 = l;
    let selenaLib40 = 0.3;
    let selenaLib41 = 3.0;
    let selenaLib42 = u.thickness;
    let selenaLib43 = normalize((selenaLib39 + (selenaLib37 * selenaLib40)));
    let selenaLib44 = (pow(clamp(dot(selenaLib38, (-selenaLib43)), 0.0, 1.0), max(selenaLib41, 1.0)) * (1.0 - clamp(selenaLib42, 0.0, 1.0)));
    let selenaLib45 = i;
    let selenaLib46 = select(0.0, 1.0, (f32(selenaLib45) < u.lightCount));
    let back = (selenaLib44 * selenaLib46);
    let selenaLib47 = n;
    let selenaLib48 = l;
    let selenaLib49 = u.scatter;
    let selenaLib50 = 0.35;
    let selenaLib51 = clamp((1.0 - (abs(dot(selenaLib47, selenaLib48)) / max(selenaLib50, 0.0001))), 0.0, 1.0);
    let selenaLib52 = (selenaLib49 * selenaLib51);
    let selenaLib53 = i;
    let selenaLib54 = select(0.0, 1.0, (f32(selenaLib53) < u.lightCount));
    let selenaLib55 = i;
    let selenaLib56 = n;
    let selenaLib57 = v;
    let selenaLib58 = in.vWorldPos;
    let selenaLib59 = 24.0;
    let selenaLib60 = selenaLib55;
    let selenaLib61 = u.lights[(selenaLib60 * 4i)].w;
    let selenaLib62 = selenaLib61;
    let selenaLib63 = selenaLib55;
    let selenaLib64 = selenaLib58;
    let selenaLib65 = u.lights[(selenaLib63 * 4i)];
    let selenaLib66 = u.lights[((selenaLib63 * 4i) + 1i)];
    let selenaLib67 = (selenaLib65.xyz - selenaLib64);
    let selenaLib68 = ((selenaLib65.w == 2.0) || (selenaLib65.w == 3.0));
    let selenaLib69 = select(normalize(((-selenaLib66.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib67 / max(length(selenaLib67), 0.0001)), selenaLib68);
    let selenaLib70 = selenaLib69;
    let selenaLib71 = normalize((selenaLib70 + selenaLib57));
    let selenaLib72 = select(0.0, 1.0, (((dot(selenaLib56, selenaLib70) > 0.0) && (selenaLib62 != 0.0)) && (selenaLib62 != 4.0)));
    let selenaLib73 = ((selenaLib59 + 8.0) / 25.1327412287);
    let selenaLib74 = selenaLib55;
    let selenaLib75 = selenaLib56;
    let selenaLib76 = selenaLib58;
    let selenaLib77 = u.lights[(selenaLib74 * 4i)];
    let selenaLib78 = u.lights[((selenaLib74 * 4i) + 1i)];
    let selenaLib79 = u.lights[((selenaLib74 * 4i) + 2i)];
    let selenaLib80 = u.lights[((selenaLib74 * 4i) + 3i)];
    let selenaLib81 = length((selenaLib77.xyz - selenaLib76));
    let selenaLib82 = (pow(clamp((1.0 - pow((selenaLib81 / max(selenaLib78.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib81 * selenaLib81), 0.0001));
    let selenaLib83 = (1.0 / max(pow(max(selenaLib81, 0.0001), selenaLib79.w), 0.0001));
    let selenaLib84 = select(1.0, select(selenaLib83, selenaLib82, (selenaLib78.w > 0.0)), ((selenaLib77.w == 2.0) || (selenaLib77.w == 3.0)));
    let selenaLib85 = selenaLib74;
    let selenaLib86 = selenaLib76;
    let selenaLib87 = u.lights[(selenaLib85 * 4i)];
    let selenaLib88 = u.lights[((selenaLib85 * 4i) + 1i)];
    let selenaLib89 = (selenaLib87.xyz - selenaLib86);
    let selenaLib90 = ((selenaLib87.w == 2.0) || (selenaLib87.w == 3.0));
    let selenaLib91 = select(normalize(((-selenaLib88.xyz) + vec3<f32>(0.0, 0.000001, 0.0))), (selenaLib89 / max(length(selenaLib89), 0.0001)), selenaLib90);
    let selenaLib92 = dot((-selenaLib91), normalize((selenaLib78.xyz + vec3<f32>(0.0, 0.000001, 0.0))));
    let selenaLib93 = select(1.0, smoothstep(selenaLib80.x, selenaLib80.y, selenaLib92), (selenaLib77.w == 3.0));
    let selenaLib94 = select(selenaLib79.xyz, mix(selenaLib80.xyz, selenaLib79.xyz, clamp(((selenaLib75.y * 0.5) + 0.5), 0.0, 1.0)), (selenaLib77.w == 4.0));
    let selenaLib95 = selenaLib74;
    let selenaLib96 = select(0.0, 1.0, (f32(selenaLib95) < u.lightCount));
    let selenaLib97 = (((selenaLib94 * selenaLib84) * selenaLib93) * selenaLib96);
    let selenaLib98 = (((selenaLib97 * pow(max(dot(selenaLib56, selenaLib71), 0.0), selenaLib59)) * selenaLib73) * selenaLib72);
    lit = ((lit + (radiance * (((u.albedo * soft) + (selenaLib52 * selenaLib54)) + (u.scatter * back)))) + (0.15 * selenaLib98));
  }
  let selenaLib99 = lit;
  let selenaLib100 = in.vWorldPos;
  let selenaLib101 = selenaLib100;
  let selenaLib102 = (length((selenaLib101 - u.cameraPos)) * u.fog.w);
  let selenaLib103 = clamp(exp((-(selenaLib102 * selenaLib102))), 0.0, 1.0);
  let selenaLib104 = mix(u.fog.xyz, selenaLib99, selenaLib103);
  return vec4<f32>(selenaLib104, 1.0);
}
