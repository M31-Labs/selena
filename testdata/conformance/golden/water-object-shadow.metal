#include <metal_stdlib>
using namespace metal;

struct UserUniforms {
  float objectKind;
  float objectEnabled;
  float poolWidth;
  float poolLength;
  float objectRadius;
  float objectHalfX;
  float objectHalfZ;
  float3 lightDir;
  float objectCenterX;
  float objectCenterZ;
};

struct PostOut {
  float4 pos [[position]];
  float2 v_uv;
};

vertex PostOut vertexMain(uint vid [[vertex_id]]) {
  const float2 pos[3] = { {-1,-1},{3,-1},{-1,3} };
  const float2 uvs[3] = { {0,1},{2,1},{0,-1} };
  PostOut out;
  out.pos  = float4(pos[vid], 0, 1);
  out.v_uv = uvs[vid];
  return out;
}

fragment float4 fragmentMain(PostOut in [[stage_in]], texture2d<float> _sceneColorTex [[texture(0)]], sampler _sceneColorSamp [[sampler(0)]], depth2d<float> _sceneDepthTex [[texture(2)]], sampler _sceneDepthSamp [[sampler(2)]], constant UserUniforms& u [[buffer(1)]]) {
  float3 lnorm = normalize(u.lightDir);
  float uvX = clamp((in.v_uv.x - (lnorm.x * 0.025)), 0.0, 1.0);
  float uvY = clamp((in.v_uv.y + (lnorm.z * 0.025)), 0.0, 1.0);
  float2 centerUV = float2(((u.objectCenterX * 0.5) + 0.5), ((u.objectCenterZ * 0.5) + 0.5));
  float2 aspect = float2(max((u.poolWidth / max(u.poolLength, 0.001)), 0.001), 1.0);
  float dd = length((float2((uvX - centerUV.x), (uvY - centerUV.y)) * aspect));
  float sphR = max((u.objectRadius * 0.55), 0.018);
  float cubeR = max((max(u.objectHalfX, u.objectHalfZ) * 0.6), sphR);
  float radius = ((u.objectKind > 1.5) ? cubeR : sphR);
  float mask = 0.0;
  float core = 0.0;
  if (((u.objectKind >= 0.5) && (u.objectEnabled > 0.0))) {
    mask = (1.0 - smoothstep(radius, (radius + max((radius * 1.2), 0.02)), dd));
    core = (1.0 - smoothstep((radius * 0.38), radius, dd));
  }
  float shadow = (mask * (0.42 + (core * 0.58)));
  return float4(float3(shadow, shadow, shadow), 1.0);
}
