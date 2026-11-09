#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 baseColor;
  float3 edgeColor;
  float progress;
  float noiseScale;
  float edgeWidth;
  float3 sunDir;
  float3 sunColor;
  float3 ambient;
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
  float3 selenaLib0 = (in.vWorldPos * u.noiseScale);
  float2 selenaLib1 = selenaLib0.xy;
  float2 selenaLib2 = floor(selenaLib1);
  float2 selenaLib3 = fract(selenaLib1);
  float2 selenaLib4 = ((selenaLib3 * selenaLib3) * (3.0 - (2.0 * selenaLib3)));
  float2 selenaLib5 = selenaLib2;
  float selenaLib6 = fract((sin(dot(selenaLib5, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib7 = selenaLib6;
  float2 selenaLib8 = (selenaLib2 + float2(1.0, 0.0));
  float selenaLib9 = fract((sin(dot(selenaLib8, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib10 = selenaLib9;
  float2 selenaLib11 = (selenaLib2 + float2(0.0, 1.0));
  float selenaLib12 = fract((sin(dot(selenaLib11, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib13 = selenaLib12;
  float2 selenaLib14 = (selenaLib2 + float2(1.0, 1.0));
  float selenaLib15 = fract((sin(dot(selenaLib14, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib16 = selenaLib15;
  float selenaLib17 = mix(mix(selenaLib7, selenaLib10, selenaLib4.x), mix(selenaLib13, selenaLib16, selenaLib4.x), selenaLib4.y);
  float2 selenaLib18 = (selenaLib0.zx + float2(17.0, 31.0));
  float2 selenaLib19 = floor(selenaLib18);
  float2 selenaLib20 = fract(selenaLib18);
  float2 selenaLib21 = ((selenaLib20 * selenaLib20) * (3.0 - (2.0 * selenaLib20)));
  float2 selenaLib22 = selenaLib19;
  float selenaLib23 = fract((sin(dot(selenaLib22, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib24 = selenaLib23;
  float2 selenaLib25 = (selenaLib19 + float2(1.0, 0.0));
  float selenaLib26 = fract((sin(dot(selenaLib25, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib27 = selenaLib26;
  float2 selenaLib28 = (selenaLib19 + float2(0.0, 1.0));
  float selenaLib29 = fract((sin(dot(selenaLib28, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib30 = selenaLib29;
  float2 selenaLib31 = (selenaLib19 + float2(1.0, 1.0));
  float selenaLib32 = fract((sin(dot(selenaLib31, float2(127.1, 311.7))) * 43758.5453));
  float selenaLib33 = selenaLib32;
  float selenaLib34 = mix(mix(selenaLib24, selenaLib27, selenaLib21.x), mix(selenaLib30, selenaLib33, selenaLib21.x), selenaLib21.y);
  float selenaLib35 = ((selenaLib17 + selenaLib34) * 0.5);
  float selenaLib36 = selenaLib35;
  float selenaLib37 = u.progress;
  float selenaLib38 = ((clamp(selenaLib36, 0.0, 1.0) - (clamp(selenaLib37, 0.0, 1.0) * 1.02)) + 0.01);
  float mask = selenaLib38;
  if ((mask < 0.0)) {
    discard_fragment();
  }
  float3 n = normalize(in.vWorldNormal);
  float3 lit = (u.baseColor * (u.ambient + (u.sunColor * max(dot(n, normalize(u.sunDir)), 0.0))));
  float selenaLib39 = mask;
  float selenaLib40 = u.edgeWidth;
  float selenaLib41 = (1.0 - smoothstep(0.0, max(selenaLib40, 0.0001), selenaLib39));
  return float4((lit + ((u.edgeColor * selenaLib41) * ((u.progress > 0.0) ? 1.0 : 0.0))), 1.0);
}
