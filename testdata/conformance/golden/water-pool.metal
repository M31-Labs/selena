#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float poolWidth;
  float poolLength;
  float poolHeight;
  float cornerRadius;
  float poolShape;
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
  float hw = max(u.poolWidth, 0.001);
  float hl = max(u.poolLength, 0.001);
  float flY = (-max(u.poolHeight, 0.001));
  float rimY = max((u.poolHeight * 0.1667), 0.025);
  float maxCornerRadius = max(0.0, (min(hw, hl) - 0.001));
  float cr = clamp(u.cornerRadius, 0.0, maxCornerRadius);
  bool roundedActive = ((u.poolShape > 0.5) && (cr > 0.0001));
  float wx = 0.0;
  float wy = 0.0;
  float wz = 0.0;
  float nx = 0.0;
  float ny = 1.0;
  float nz = 0.0;
  float tileX = 0.0;
  float tileY = 0.0;
  float faceOut = 0.0;
  if (roundedActive) {
    float insetX = max((hw - cr), 0.001);
    float insetY = max((hl - cr), 0.001);
    if ((fi < 132.0)) {
      float triF = floor((fi / 3.0));
      float triCorner = (fi - (triF * 3.0));
      float px = 0.0;
      float py = 0.0;
      if ((triCorner == 1.0)) {
        float c1Raw = (triF + 1.0);
        float c1SegF = floor((c1Raw / 44.0));
        float c1Idx = (c1Raw - (c1SegF * 44.0));
        float c1CornF = floor((c1Idx / 11.0));
        float c1Corn = min(c1CornF, 3.0);
        float c1Local = (c1Idx - (c1Corn * 11.0));
        float c1SignX = (((c1Corn == 1.0) || (c1Corn == 2.0)) ? (-1.0) : 1.0);
        float c1SignY = ((c1Corn >= 2.0) ? (-1.0) : 1.0);
        float c1Theta = ((c1Corn * 1.57079632679) + ((c1Local / 10.0) * 1.57079632679));
        px = ((c1SignX * insetX) + (cos(c1Theta) * cr));
        py = ((c1SignY * insetY) + (sin(c1Theta) * cr));
      } else {
        if ((triCorner == 2.0)) {
          float c2SegF = floor((triF / 44.0));
          float c2Idx = (triF - (c2SegF * 44.0));
          float c2CornF = floor((c2Idx / 11.0));
          float c2Corn = min(c2CornF, 3.0);
          float c2Local = (c2Idx - (c2Corn * 11.0));
          float c2SignX = (((c2Corn == 1.0) || (c2Corn == 2.0)) ? (-1.0) : 1.0);
          float c2SignY = ((c2Corn >= 2.0) ? (-1.0) : 1.0);
          float c2Theta = ((c2Corn * 1.57079632679) + ((c2Local / 10.0) * 1.57079632679));
          px = ((c2SignX * insetX) + (cos(c2Theta) * cr));
          py = ((c2SignY * insetY) + (sin(c2Theta) * cr));
        }
      }
      wx = px;
      wy = flY;
      wz = py;
      tileX = (px * 0.42);
      tileY = (py * 0.42);
      faceOut = 0.0;
    } else {
      float localIndex = (fi - 132.0);
      float segRaw = floor((localIndex / 6.0));
      float segSegF = floor((segRaw / 44.0));
      float segment = (segRaw - (segSegF * 44.0));
      float wallCorner = (localIndex - (segRaw * 6.0));
      float quadU = ((((wallCorner == 1.0) || (wallCorner == 2.0)) || (wallCorner == 4.0)) ? 1.0 : 0.0);
      float quadV = ((((wallCorner == 2.0) || (wallCorner == 4.0)) || (wallCorner == 5.0)) ? 1.0 : 0.0);
      float aCornF = min(floor((segment / 11.0)), 3.0);
      float aLocal = (segment - (aCornF * 11.0));
      float aSignX = (((aCornF == 1.0) || (aCornF == 2.0)) ? (-1.0) : 1.0);
      float aSignY = ((aCornF >= 2.0) ? (-1.0) : 1.0);
      float aTheta = ((aCornF * 1.57079632679) + ((aLocal / 10.0) * 1.57079632679));
      float aPX = ((aSignX * insetX) + (cos(aTheta) * cr));
      float aPY = ((aSignY * insetY) + (sin(aTheta) * cr));
      float bRaw = (segment + 1.0);
      float bSegF = floor((bRaw / 44.0));
      float bIdx = (bRaw - (bSegF * 44.0));
      float bCornF = min(floor((bIdx / 11.0)), 3.0);
      float bLocal = (bIdx - (bCornF * 11.0));
      float bSignX = (((bCornF == 1.0) || (bCornF == 2.0)) ? (-1.0) : 1.0);
      float bSignY = ((bCornF >= 2.0) ? (-1.0) : 1.0);
      float bTheta = ((bCornF * 1.57079632679) + ((bLocal / 10.0) * 1.57079632679));
      float bPX = ((bSignX * insetX) + (cos(bTheta) * cr));
      float bPY = ((bSignY * insetY) + (sin(bTheta) * cr));
      float2 point = float2(mix(aPX, bPX, quadU), mix(aPY, bPY, quadU));
      float2 inset = float2(insetX, insetY);
      float2 absPoint = abs(point);
      float2 outward = float2(0.0, 1.0);
      if ((((absPoint.x > insetX) && (absPoint.y > insetY)) && (cr > 0.0001))) {
        outward = normalize((point - (sign(point) * inset)));
      } else {
        if (((absPoint.x / max(hw, 0.001)) > (absPoint.y / max(hl, 0.001)))) {
          outward = float2(sign(point.x), 0.0);
        } else {
          outward = float2(0.0, sign(point.y));
        }
      }
      wx = point.x;
      wy = mix(flY, rimY, quadV);
      wz = point.y;
      nx = (-outward.x);
      ny = 0.0;
      nz = (-outward.y);
      tileX = ((segment + quadU) * 0.18);
      tileY = (wy * 0.72);
      faceOut = 5.0;
    }
  } else {
    float fv = floor((fi / 6.0));
    float faceF = min(fv, 4.0);
    float cu = (fi - (fv * 6.0));
    float uf = ((cu == 1.0) ? 1.0 : ((cu == 2.0) ? 1.0 : ((cu == 4.0) ? 1.0 : 0.0)));
    float vf = ((cu == 2.0) ? 1.0 : ((cu == 4.0) ? 1.0 : ((cu == 5.0) ? 1.0 : 0.0)));
    wx = ((faceF == 2.0) ? mix(hw, (-hw), uf) : ((faceF == 3.0) ? hw : ((faceF == 4.0) ? (-hw) : mix((-hw), hw, uf))));
    wy = ((faceF == 0.0) ? flY : mix(flY, rimY, vf));
    wz = ((faceF == 1.0) ? hl : ((faceF == 2.0) ? (-hl) : ((faceF == 3.0) ? mix(hl, (-hl), uf) : ((faceF == 4.0) ? mix((-hl), hl, uf) : mix((-hl), hl, vf)))));
    nx = ((faceF == 3.0) ? (-1.0) : ((faceF == 4.0) ? 1.0 : 0.0));
    ny = ((faceF == 0.0) ? 1.0 : 0.0);
    nz = ((faceF == 1.0) ? (-1.0) : ((faceF == 2.0) ? 1.0 : 0.0));
    tileX = ((faceF == 3.0) ? (wz * 0.42) : ((faceF == 4.0) ? (wz * 0.42) : (wx * 0.42)));
    tileY = ((faceF == 0.0) ? (wz * 0.42) : (wy * 0.72));
    faceOut = faceF;
  }
  float duw = max((u.poolWidth * 2.0), 0.001);
  float dul = max((u.poolLength * 2.0), 0.001);
  out.vWorldPos = float3(wx, wy, wz);
  out.vNormal = float3(nx, ny, nz);
  out.vTileUV = float2(tileX, tileY);
  out.vWaterUV = float2(((wx / duw) + 0.5), ((wz / dul) + 0.5));
  out.vFace = faceOut;
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
