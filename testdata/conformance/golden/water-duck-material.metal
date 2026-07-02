#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float poolHeight;
  float4 baseColor;
  float isTexturePass;
  float texturePassMode;
  float3 lightDir;
};

struct StateGrid {
  uint gridWidth;
  uint gridHeight;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
  float2 uv [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float3 worldPos;
  float2 vUv;
  float3 vNormal;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  VertexOut out;
  out.worldPos = in.position;
  out.vUv = in.uv;
  out.vNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> modelTexture [[texture(0)]], sampler modelTextureSampler [[sampler(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  float4 info = _inState[min(uint((in.vUv).x * float(_stateGrid.gridWidth)) + uint((in.vUv).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float waterHeight = (info.x * u.poolHeight);
  float caustic = info.y;
  bool belowWater = (in.worldPos.y < waterHeight);
  if ((((u.isTexturePass > 0.5) && (u.texturePassMode == 2.0)) && belowWater)) {
    discard_fragment();
  }
  float3 lightN = normalize(u.lightDir);
  float3 refr = refract((-lightN), float3(0.0, 1.0, 0.0), (1.0 / 1.333));
  float3 n = normalize(in.vNormal);
  float3 albedo = (modelTexture.sample(modelTextureSampler, in.vUv).rgb * u.baseColor.rgb);
  bool submerged = (in.worldPos.y < waterHeight);
  float diffuse = (max(dot((-refr), n), 0.0) * 0.6);
  if (submerged) {
    diffuse = ((diffuse * caustic) * 4.0);
  }
  float3 underwater = float3(0.4, 0.9, 1.0);
  float3 col = (albedo * (0.4 + diffuse));
  if (submerged) {
    col = ((col * underwater) * 1.2);
  }
  return float4(col, u.baseColor.a);
}
