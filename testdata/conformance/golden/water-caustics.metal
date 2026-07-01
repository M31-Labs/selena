#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float poolWidth;
  float poolLength;
  float poolHeight;
  float normalScale;
  float opticsEnable;
  float resolution;
  float time;
  float objectKind;
  float objectCount;
  float3 lightDir;
  float3 objectCenter;
  float4 objectHalfRadius;
  float4 spheres[32];
};

struct StateGrid {
  uint gridWidth;
  uint gridHeight;
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
};

vertex VertexOut vertexMain(uint vertexIndex [[vertex_id]], constant Uniforms& u [[buffer(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  VertexOut out;
  float fi = float(vertexIndex);
  float ox = ((((fi == 1.0) || (fi == 2.0)) || (fi == 4.0)) ? 1.0 : 0.0);
  float oy = ((((fi == 2.0) || (fi == 4.0)) || (fi == 5.0)) ? 1.0 : 0.0);
  out.vUv = float2(ox, oy);
  out.position = float4(((ox * 2.0) - 1.0), ((oy * 2.0) - 1.0), 0.0, 1.0);
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], constant StateGrid& _stateGrid [[buffer(1)]], const device float4* _inState [[buffer(2)]]) {
  float2 uv = clamp(in.vUv, float2(0.0, 0.0), float2(1.0, 1.0));
  float texel = (1.0 / max(u.resolution, 1.0));
  float4 c = _inState[min(uint((uv).x * float(_stateGrid.gridWidth)) + uint((uv).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float4 e = _inState[min(uint(((uv + float2(texel, 0.0))).x * float(_stateGrid.gridWidth)) + uint(((uv + float2(texel, 0.0))).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float4 wv = _inState[min(uint(((uv - float2(texel, 0.0))).x * float(_stateGrid.gridWidth)) + uint(((uv - float2(texel, 0.0))).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float4 nn = _inState[min(uint(((uv + float2(0.0, texel))).x * float(_stateGrid.gridWidth)) + uint(((uv + float2(0.0, texel))).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float4 ss = _inState[min(uint(((uv - float2(0.0, texel))).x * float(_stateGrid.gridWidth)) + uint(((uv - float2(0.0, texel))).y * float(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  float3 ldir = normalize(u.lightDir);
  float3 waterNormal = normalize(float3((c.z * u.normalScale), 1.0, (c.w * u.normalScale)));
  float3 causticNormal = normalize(float3(((c.z * u.normalScale) * 0.5), 1.0, ((c.w * u.normalScale) * 0.5)));
  float3 refracted = refract((-ldir), waterNormal, (1.0 / 1.333));
  float3 causticRay = refract((-ldir), causticNormal, (1.0 / 1.333));
  float3 flatRay = refract((-ldir), float3(0.0, 1.0, 0.0), (1.0 / 1.333));
  float3 origin = float3((((uv.x - 0.5) * u.poolWidth) * 2.0), 0.0, (((uv.y - 0.5) * u.poolLength) * 2.0));
  float3 originH = float3(origin.x, (c.x * u.poolHeight), origin.z);
  float3 oldPos = (origin + (flatRay * (((-u.poolHeight) - origin.y) / ((abs(flatRay.y) > 0.0001) ? flatRay.y : ((flatRay.y >= 0.0) ? 0.0001 : (-0.0001))))));
  float3 newPos = (originH + (causticRay * (((-u.poolHeight) - originH.y) / ((abs(causticRay.y) > 0.0001) ? causticRay.y : ((causticRay.y >= 0.0) ? 0.0001 : (-0.0001))))));
  float oldArea = max((length(dfdx(oldPos)) * length(dfdy(oldPos))), 0.000001);
  float newArea = max((length(dfdx(newPos)) * length(dfdy(newPos))), 0.000001);
  float convergence = abs(((((e.x + wv.x) + nn.x) + ss.x) - (c.x * 4.0)));
  float3 slopeRay = normalize(float3((-refracted.x), max(refracted.y, 0.05), (-refracted.z)));
  float slopeFocus = max(0.0, dot(slopeRay, waterNormal));
  float shimmer = (0.5 + (0.5 * sin(((((uv.x * 41.0) + (uv.y * 37.0)) + (u.time * 2.4)) + (c.x * 180.0)))));
  float areaFocus = clamp(((oldArea / newArea) * 0.2), 0.0, 4.0);
  float slopeMag = length(c.zw);
  float intensity = (areaFocus * (0.68 + (0.32 * smoothstep(0.001, 0.028, ((convergence * 0.72) + (slopeMag * 0.035))))));
  intensity = ((intensity * (0.52 + (0.48 * shimmer))) * (0.58 + (0.42 * slopeFocus)));
  float2 centerUV = float2(((u.objectCenter.x * 0.5) + 0.5), ((u.objectCenter.z * 0.5) + 0.5));
  float2 aspect = float2(max((u.poolWidth / max(u.poolLength, 0.001)), 0.001), 1.0);
  float compMask = 0.0;
  for (int i = 0; (i < 32); i = (i + 1)) {
    if (((u.objectKind >= 2.5) && (float(i) < u.objectCount))) {
      float4 sp = u.spheres[i];
      float2 suv = (centerUV + float2((sp.x * 0.5), (sp.z * 0.5)));
      float rad = max((sp.w * 0.72), 0.012);
      float dd = length(((uv - suv) * aspect));
      compMask = max(compMask, (1.0 - smoothstep(rad, (rad + max((rad * 1.25), 0.018)), dd)));
    }
  }
  float singleR0 = max((u.objectHalfRadius.w * 0.55), 0.018);
  float singleRC = max((max(u.objectHalfRadius.x, u.objectHalfRadius.z) * 0.6), singleR0);
  float singleR = ((u.objectKind > 1.5) ? singleRC : singleR0);
  float singleD = length(((uv - centerUV) * aspect));
  float singleMask = (1.0 - smoothstep(singleR, (singleR + max((singleR * 1.2), 0.02)), singleD));
  float maskRaw = ((u.objectKind >= 2.5) ? compMask : singleMask);
  bool maskOn = ((u.objectKind >= 0.5) && (u.opticsEnable > 0.0));
  float shadowMask = (maskOn ? maskRaw : 0.0);
  float sphereShadow = ((((u.objectKind >= 0.5) && (u.objectKind < 1.5)) && (u.opticsEnable > 0.0)) ? (1.0 - mix(1.0, clamp((1.0 / (1.0 + exp((-(1.0 + ((dot(cross(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), flatRay), cross(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), flatRay)) - 1.0) / max((0.05 + (dot(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), (-flatRay)) * 0.025)), 0.0001))))))), 0.0, 1.0), clamp((dot(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), (-flatRay)) * 2.0), 0.0, 1.0))) : 0.0);
  float3 shadowRay = (-flatRay);
  float3 sright = normalize(cross(shadowRay, float3(0.0, 1.0, 0.0)));
  float3 sup = normalize(cross(sright, shadowRay));
  bool cubeActive = (((u.objectKind >= 1.5) && (u.objectKind < 2.5)) && (u.opticsEnable > 0.0));
  float3 cubeHalf = u.objectHalfRadius.xyz;
  float cubeOcc = 0.0;
  for (int cx = 0; (cx < 3); cx = (cx + 1)) {
    for (int cy = 0; (cy < 3); cy = (cy + 1)) {
      float fx = (float(cx) - 1.0);
      float fy = (float(cy) - 1.0);
      float3 so = ((newPos + ((sright * fx) * 0.025)) + ((sup * fy) * 0.025));
      cubeOcc = (cubeOcc + (step(0.0, float2(max(max(min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z), min(min(max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z)).y) * step(float2(max(max(min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z), min(min(max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z)).x, float2(max(max(min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), min((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z), min(min(max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), max((((u.objectCenter - max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((u.objectCenter + max(cubeHalf, float3(0.0001, 0.0001, 0.0001))) - so) * float3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z)).y)));
    }
  }
  float cubeShadow = (cubeActive ? (cubeOcc / 9.0) : 0.0);
  float shadow = max(max(shadowMask, sphereShadow), cubeShadow);
  float lit = (intensity * (1.0 - (shadow * 0.82)));
  float3 warm = float3(1.0, 0.78, 0.42);
  float3 cool = float3(0.44, 0.95, 1.0);
  return float4((mix(cool, warm, clamp((lit * 1.8), 0.0, 1.0)) * lit), 1.0);
}
