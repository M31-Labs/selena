#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  packed_float3 albedo;
  char _selenaPad0[4];
  packed_float3 scatter;
  float wrap;
  float thickness;
  char _selenaPad1[12];
  packed_float3 cameraPos;
  char _selenaPad2[4];
  packed_float3 ambient;
  char _selenaPad3[4];
  float4 fog;
  float lightCount;
  char _selenaPad4[12];
  float4 lights[32];
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
};

struct VertexOut {
  float4 position [[position]];
  float3 vWorldNormal;
  float3 vWorldPos;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.vWorldPos = in.position;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float3 n = normalize(in.vWorldNormal);
  float3 v = normalize((float3(u.cameraPos) - in.vWorldPos));
  float3 lit = (float3(u.ambient) * float3(u.albedo));
  for (int i = 0; (i < 8); i = (i + 1)) {
    int selenaLib0 = i;
    float3 selenaLib1 = in.vWorldPos;
    float4 selenaLib2 = u.lights[(selenaLib0 * 4)];
    float4 selenaLib3 = u.lights[((selenaLib0 * 4) + 1)];
    float3 selenaLib4 = (selenaLib2.xyz - selenaLib1);
    bool selenaLib5 = ((selenaLib2.w == 2.0) || (selenaLib2.w == 3.0));
    float3 selenaLib6 = (selenaLib5 ? (selenaLib4 / max(length(selenaLib4), 0.0001)) : normalize(((-selenaLib3.xyz) + float3(0.0, 0.000001, 0.0))));
    float3 l = selenaLib6;
    int selenaLib7 = i;
    float3 selenaLib8 = n;
    float3 selenaLib9 = in.vWorldPos;
    float4 selenaLib10 = u.lights[(selenaLib7 * 4)];
    float4 selenaLib11 = u.lights[((selenaLib7 * 4) + 1)];
    float4 selenaLib12 = u.lights[((selenaLib7 * 4) + 2)];
    float4 selenaLib13 = u.lights[((selenaLib7 * 4) + 3)];
    float selenaLib14 = length((selenaLib10.xyz - selenaLib9));
    float selenaLib15 = (pow(clamp((1.0 - pow((selenaLib14 / max(selenaLib11.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib14 * selenaLib14), 0.0001));
    float selenaLib16 = (1.0 / max(pow(max(selenaLib14, 0.0001), selenaLib12.w), 0.0001));
    float selenaLib17 = (((selenaLib10.w == 2.0) || (selenaLib10.w == 3.0)) ? ((selenaLib11.w > 0.0) ? selenaLib15 : selenaLib16) : 1.0);
    int selenaLib18 = selenaLib7;
    float3 selenaLib19 = selenaLib9;
    float4 selenaLib20 = u.lights[(selenaLib18 * 4)];
    float4 selenaLib21 = u.lights[((selenaLib18 * 4) + 1)];
    float3 selenaLib22 = (selenaLib20.xyz - selenaLib19);
    bool selenaLib23 = ((selenaLib20.w == 2.0) || (selenaLib20.w == 3.0));
    float3 selenaLib24 = (selenaLib23 ? (selenaLib22 / max(length(selenaLib22), 0.0001)) : normalize(((-selenaLib21.xyz) + float3(0.0, 0.000001, 0.0))));
    float selenaLib25 = dot((-selenaLib24), normalize((selenaLib11.xyz + float3(0.0, 0.000001, 0.0))));
    float selenaLib26 = ((selenaLib10.w == 3.0) ? smoothstep(selenaLib13.x, selenaLib13.y, selenaLib25) : 1.0);
    float3 selenaLib27 = ((selenaLib10.w == 4.0) ? mix(selenaLib13.xyz, selenaLib12.xyz, clamp(((selenaLib8.y * 0.5) + 0.5), 0.0, 1.0)) : selenaLib12.xyz);
    int selenaLib28 = selenaLib7;
    float selenaLib29 = ((float(selenaLib28) < u.lightCount) ? 1.0 : 0.0);
    float3 selenaLib30 = (((selenaLib27 * selenaLib17) * selenaLib26) * selenaLib29);
    float3 radiance = selenaLib30;
    float3 selenaLib31 = n;
    float3 selenaLib32 = l;
    float selenaLib33 = u.wrap;
    float selenaLib34 = clamp(((dot(selenaLib31, selenaLib32) + selenaLib33) / (1.0 + selenaLib33)), 0.0, 1.0);
    int selenaLib35 = i;
    float selenaLib36 = ((float(selenaLib35) < u.lightCount) ? 1.0 : 0.0);
    float soft = (selenaLib34 * selenaLib36);
    float3 selenaLib37 = n;
    float3 selenaLib38 = v;
    float3 selenaLib39 = l;
    float selenaLib40 = 0.3;
    float selenaLib41 = 3.0;
    float selenaLib42 = u.thickness;
    float3 selenaLib43 = normalize((selenaLib39 + (selenaLib37 * selenaLib40)));
    float selenaLib44 = (pow(clamp(dot(selenaLib38, (-selenaLib43)), 0.0, 1.0), max(selenaLib41, 1.0)) * (1.0 - clamp(selenaLib42, 0.0, 1.0)));
    int selenaLib45 = i;
    float selenaLib46 = ((float(selenaLib45) < u.lightCount) ? 1.0 : 0.0);
    float back = (selenaLib44 * selenaLib46);
    float3 selenaLib47 = n;
    float3 selenaLib48 = l;
    float3 selenaLib49 = float3(u.scatter);
    float selenaLib50 = 0.35;
    float selenaLib51 = clamp((1.0 - (abs(dot(selenaLib47, selenaLib48)) / max(selenaLib50, 0.0001))), 0.0, 1.0);
    float3 selenaLib52 = (selenaLib49 * selenaLib51);
    int selenaLib53 = i;
    float selenaLib54 = ((float(selenaLib53) < u.lightCount) ? 1.0 : 0.0);
    int selenaLib55 = i;
    float3 selenaLib56 = n;
    float3 selenaLib57 = v;
    float3 selenaLib58 = in.vWorldPos;
    float selenaLib59 = 24.0;
    int selenaLib60 = selenaLib55;
    float selenaLib61 = u.lights[(selenaLib60 * 4)].w;
    float selenaLib62 = selenaLib61;
    int selenaLib63 = selenaLib55;
    float3 selenaLib64 = selenaLib58;
    float4 selenaLib65 = u.lights[(selenaLib63 * 4)];
    float4 selenaLib66 = u.lights[((selenaLib63 * 4) + 1)];
    float3 selenaLib67 = (selenaLib65.xyz - selenaLib64);
    bool selenaLib68 = ((selenaLib65.w == 2.0) || (selenaLib65.w == 3.0));
    float3 selenaLib69 = (selenaLib68 ? (selenaLib67 / max(length(selenaLib67), 0.0001)) : normalize(((-selenaLib66.xyz) + float3(0.0, 0.000001, 0.0))));
    float3 selenaLib70 = selenaLib69;
    float3 selenaLib71 = normalize((selenaLib70 + selenaLib57));
    float selenaLib72 = ((((dot(selenaLib56, selenaLib70) > 0.0) && (selenaLib62 != 0.0)) && (selenaLib62 != 4.0)) ? 1.0 : 0.0);
    float selenaLib73 = ((selenaLib59 + 8.0) / 25.1327412287);
    int selenaLib74 = selenaLib55;
    float3 selenaLib75 = selenaLib56;
    float3 selenaLib76 = selenaLib58;
    float4 selenaLib77 = u.lights[(selenaLib74 * 4)];
    float4 selenaLib78 = u.lights[((selenaLib74 * 4) + 1)];
    float4 selenaLib79 = u.lights[((selenaLib74 * 4) + 2)];
    float4 selenaLib80 = u.lights[((selenaLib74 * 4) + 3)];
    float selenaLib81 = length((selenaLib77.xyz - selenaLib76));
    float selenaLib82 = (pow(clamp((1.0 - pow((selenaLib81 / max(selenaLib78.w, 0.0001)), 4.0)), 0.0, 1.0), 2.0) / max((selenaLib81 * selenaLib81), 0.0001));
    float selenaLib83 = (1.0 / max(pow(max(selenaLib81, 0.0001), selenaLib79.w), 0.0001));
    float selenaLib84 = (((selenaLib77.w == 2.0) || (selenaLib77.w == 3.0)) ? ((selenaLib78.w > 0.0) ? selenaLib82 : selenaLib83) : 1.0);
    int selenaLib85 = selenaLib74;
    float3 selenaLib86 = selenaLib76;
    float4 selenaLib87 = u.lights[(selenaLib85 * 4)];
    float4 selenaLib88 = u.lights[((selenaLib85 * 4) + 1)];
    float3 selenaLib89 = (selenaLib87.xyz - selenaLib86);
    bool selenaLib90 = ((selenaLib87.w == 2.0) || (selenaLib87.w == 3.0));
    float3 selenaLib91 = (selenaLib90 ? (selenaLib89 / max(length(selenaLib89), 0.0001)) : normalize(((-selenaLib88.xyz) + float3(0.0, 0.000001, 0.0))));
    float selenaLib92 = dot((-selenaLib91), normalize((selenaLib78.xyz + float3(0.0, 0.000001, 0.0))));
    float selenaLib93 = ((selenaLib77.w == 3.0) ? smoothstep(selenaLib80.x, selenaLib80.y, selenaLib92) : 1.0);
    float3 selenaLib94 = ((selenaLib77.w == 4.0) ? mix(selenaLib80.xyz, selenaLib79.xyz, clamp(((selenaLib75.y * 0.5) + 0.5), 0.0, 1.0)) : selenaLib79.xyz);
    int selenaLib95 = selenaLib74;
    float selenaLib96 = ((float(selenaLib95) < u.lightCount) ? 1.0 : 0.0);
    float3 selenaLib97 = (((selenaLib94 * selenaLib84) * selenaLib93) * selenaLib96);
    float3 selenaLib98 = (((selenaLib97 * pow(max(dot(selenaLib56, selenaLib71), 0.0), selenaLib59)) * selenaLib73) * selenaLib72);
    lit = ((lit + (radiance * (((float3(u.albedo) * soft) + (selenaLib52 * selenaLib54)) + (float3(u.scatter) * back)))) + (0.15 * selenaLib98));
  }
  float3 selenaLib99 = lit;
  float3 selenaLib100 = in.vWorldPos;
  float3 selenaLib101 = selenaLib100;
  float selenaLib102 = (length((selenaLib101 - float3(u.cameraPos))) * u.fog.w);
  float selenaLib103 = clamp(exp((-(selenaLib102 * selenaLib102))), 0.0, 1.0);
  float3 selenaLib104 = mix(u.fog.xyz, selenaLib99, selenaLib103);
  return float4(selenaLib104, 1.0);
}
