#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float poolWidth;
  float poolLength;
  float poolHeight;
  float3 lightDir;
};

struct StateGrid {
  uint gridWidth;
  uint gridHeight;
};

struct VertexOut {
  float4 position [[position]];
  float3 vWorldPos;
  float3 vNormal;
  float2 vTileUV;
  float2 vWaterUV;
  float vFace;
};

vertex VertexOut vertexMain(uint vertexIndex [[vertex_id]], constant Uniforms& u [[buffer(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  VertexOut out;
  float fi = float(vertexIndex);
  float fv = floor((fi / 6.0));
  float faceF = min(fv, 4.0);
  float cu = (fi - (fv * 6.0));
  float uf = ((cu == 1.0) ? 1.0 : ((cu == 2.0) ? 1.0 : ((cu == 4.0) ? 1.0 : 0.0)));
  float vf = ((cu == 2.0) ? 1.0 : ((cu == 4.0) ? 1.0 : ((cu == 5.0) ? 1.0 : 0.0)));
  float hw = max(u.poolWidth, 0.001);
  float hl = max(u.poolLength, 0.001);
  float flY = (-max(u.poolHeight, 0.001));
  float rimY = max((u.poolHeight * 0.1667), 0.025);
  float wx = ((faceF == 2.0) ? mix(hw, (-hw), uf) : ((faceF == 3.0) ? hw : ((faceF == 4.0) ? (-hw) : mix((-hw), hw, uf))));
  float wy = ((faceF == 0.0) ? flY : mix(flY, rimY, vf));
  float wz = ((faceF == 1.0) ? hl : ((faceF == 2.0) ? (-hl) : ((faceF == 3.0) ? mix(hl, (-hl), uf) : ((faceF == 4.0) ? mix((-hl), hl, uf) : mix((-hl), hl, vf)))));
  float nx = ((faceF == 3.0) ? (-1.0) : ((faceF == 4.0) ? 1.0 : 0.0));
  float ny = ((faceF == 0.0) ? 1.0 : 0.0);
  float nz = ((faceF == 1.0) ? (-1.0) : ((faceF == 2.0) ? 1.0 : 0.0));
  float tileX = ((faceF == 3.0) ? (wz * 0.42) : ((faceF == 4.0) ? (wz * 0.42) : (wx * 0.42)));
  float tileY = ((faceF == 0.0) ? (wz * 0.42) : (wy * 0.72));
  float duw = max((u.poolWidth * 2.0), 0.001);
  float dul = max((u.poolLength * 2.0), 0.001);
  out.vWorldPos = float3(wx, wy, wz);
  out.vNormal = float3(nx, ny, nz);
  out.vTileUV = float2(tileX, tileY);
  out.vWaterUV = float2(((wx / duw) + 0.5), ((wz / dul) + 0.5));
  out.vFace = faceF;
  out.position = (u.mvp * float4(wx, wy, wz, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> tileTexture [[texture(0)]], sampler tileTextureSampler [[sampler(0)]], texture2d<float> causticTexture [[texture(1)]], sampler causticTextureSampler [[sampler(1)]], texture2d<float> shadowTexture [[texture(2)]], sampler shadowTextureSampler [[sampler(2)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  float2 wuv = clamp(in.vWaterUV, float2(0.0, 0.0), float2(1.0, 1.0));
  float4 info = _inState[min(uint((wuv).x * float(_stateGrid.gridWidth)) + uint((wuv).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float wh = (info.x * u.poolHeight);
  float3 ldir = normalize(u.lightDir);
  float3 refr = refract((-ldir), float3(0.0, 1.0, 0.0), (1.0 / 1.333));
  float refY = ((abs(refr.y) > 0.05) ? refr.y : 0.05);
  float duw = max((u.poolWidth * 2.0), 0.001);
  float dul = max((u.poolLength * 2.0), 0.001);
  float projX = ((in.vWorldPos.x - ((in.vWorldPos.y * refr.x) / refY)) / duw);
  float projZ = ((in.vWorldPos.z - ((in.vWorldPos.y * refr.z) / refY)) / dul);
  float2 cUV = clamp(float2(((projX * 0.75) + 0.5), ((projZ * 0.75) + 0.5)), float2(0.0, 0.0), float2(1.0, 1.0));
  float4 tileSamp = tileTexture.sample(tileTextureSampler, in.vTileUV);
  float4 causticS = causticTexture.sample(causticTextureSampler, cUV);
  float4 shadowS = shadowTexture.sample(shadowTextureSampler, wuv);
  float3 tileRGB = tileSamp.xyz;
  float3 caustic = causticS.xyz;
  float shadowV = shadowS.x;
  float3 nrm = normalize(in.vNormal);
  float diffuse = max(dot(nrm, normalize((-refr))), 0.0);
  float below = ((in.vWorldPos.y < wh) ? 1.0 : 0.0);
  float distFade = (1.0 / max((length(in.vWorldPos) * 0.52), 1.0));
  float dryLight = (0.46 + (diffuse * 0.34));
  float causticE = dot(caustic, float3(0.34, 0.44, 0.22));
  float3 base = ((tileRGB * dryLight) * distFade);
  float3 wet = (((base * float3(0.42, 0.92, 1.0)) * (0.72 + (diffuse * 0.22))) + (caustic * (1.55 + (causticE * 0.6))));
  float3 col0 = mix(base, wet, below);
  float3 col1 = (col0 * (1.0 - (clamp(shadowV, 0.0, 1.0) * 0.62)));
  float rim = smoothstep(0.0, 0.12, in.vWorldPos.y);
  float3 rimAdd = float3(0.05, 0.035, 0.018);
  float3 col2 = mix(col1, (col1 + rimAdd), (rim * (1.0 - below)));
  return float4(float3(col2.x, col2.y, col2.z), 1.0);
}
