#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 lightDir;
  float poolHalfW;
  float poolHalfL;
};

struct VertexIn {
  float3 position [[attribute(0)]];
};

struct VertexOut {
  float4 position [[position]];
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  float3 pos = in.position;
  float3 lnorm = normalize(u.lightDir);
  float3 refr = refract((-lnorm), float3(0.0, 1.0, 0.0), (1.0 / 1.333));
  float fallY = ((refr.y >= 0.0) ? 0.0001 : (-0.0001));
  float refY = ((abs(refr.y) > 0.0001) ? refr.y : fallY);
  float pX = (0.75 * (pos.x - ((pos.y * refr.x) / refY)));
  float pZ = (0.75 * (pos.z - ((pos.y * refr.z) / refY)));
  float clipX = (pX / max(u.poolHalfW, 0.0001));
  float clipZ = (pZ / max(u.poolHalfL, 0.0001));
  out.position = float4(clipX, clipZ, 0.0, 1.0);
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  return float4(float3(1.0, 1.0, 1.0), 1.0);
}
