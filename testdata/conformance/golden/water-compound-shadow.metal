#include <metal_stdlib>
using namespace metal;

struct UserUniforms {
  float sphereCount;
  float objectEnabled;
  float objectTop;
  float poolWidth;
  float poolLength;
  float3 lightDir;
  float objectCenterX;
  float objectCenterZ;
  float4 spheres[32];
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
  float2 aspect = float2(max((u.poolWidth / max(u.poolLength, 0.001)), 0.001), 1.0);
  float mask = 0.0;
  float core = 0.0;
  for (int i = 0; (i < 32); i = (i + 1)) {
    if (((u.objectEnabled > 0.0) && (float(i) < u.sphereCount))) {
      float4 sphere = u.spheres[i];
      float sphereUVx = (((u.objectCenterX + sphere.x) * 0.5) + 0.5);
      float sphereUVy = (((u.objectCenterZ + sphere.z) * 0.5) + 0.5);
      float radius = max((sphere.w * 0.58), 0.012);
      float2 dvec = float2(((uvX - sphereUVx) * aspect.x), ((uvY - sphereUVy) * aspect.y));
      float dd = length(dvec);
      float localMask = (1.0 - smoothstep(radius, (radius + max((radius * 1.35), 0.018)), dd));
      mask = max(mask, localMask);
      core = max(core, (1.0 - smoothstep((radius * 0.42), radius, dd)));
    }
  }
  float clipped = smoothstep((-0.08), 0.16, u.objectTop);
  float shadow = ((mask * (0.42 + (0.58 * core))) * clipped);
  return float4(float3(shadow, shadow, shadow), 1.0);
}
