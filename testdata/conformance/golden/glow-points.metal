#include <metal_stdlib>
using namespace metal;

struct FrameUniforms {
  float4x4 viewMatrix;
  float4x4 projMatrix;
  float viewportWidth;
  float viewportHeight;
};

struct PointsUniforms {
  float4x4 modelMatrix;
  float4 defaultColorAndSize;
  uint4 flags;
  float4 params;
  float4 fogColor;
};

struct ParticleInstance {
  float3 position;
  float size;
  float4 color;
};

struct UserUniforms {
  float3 fogColor;
};

struct PointsOut {
  float4 clipPos [[position]];
  float3 v_color;
  float  v_fogFactor;
  float  v_alpha;
  float2 v_pointCoord;
  float  v_pointSize;
};

constant float2 _quadPos[6] = {
  {-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5},
  { 0.5, -0.5}, {0.5,  0.5}, {-0.5, 0.5},
};

vertex PointsOut vertexMain(
  uint vertexIndex   [[vertex_id]],
  uint instanceIndex [[instance_id]],
  constant FrameUniforms&  frame     [[buffer(0)]],
  constant PointsUniforms& pts       [[buffer(2)]],
  const device ParticleInstance* particles [[buffer(3)]]
, constant UserUniforms& u [[buffer(1)]]) {
  float2 quad = _quadPos[vertexIndex];
  ParticleInstance p = particles[instanceIndex];
  float3 worldPos = (pts.modelMatrix * float4(p.position, 1.0)).xyz;
  float4 viewPos  = frame.viewMatrix * float4(worldPos, 1.0);
  float rawSize = (pts.flags.y == 0u) ? pts.defaultColorAndSize.w : p.size;
  float pixelSize;
  if (pts.flags.z != 0u) {
    pixelSize = max(rawSize * (frame.viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(rawSize, 1.0);
  }
  float minPx = max(pts.fogColor.w, 0.0);
  if (minPx > 0.0) pixelSize = max(pixelSize, minPx);
  if (pts.params.w > 0.0) pixelSize = min(pixelSize, pts.params.w);
  float4 clipPos = frame.projMatrix * viewPos;
  float2 viewport = max(float2(frame.viewportWidth, frame.viewportHeight), float2(1.0));
  float ndcX = quad.x * pixelSize / viewport.x * clipPos.w * 2.0;
  float ndcY = quad.y * pixelSize / viewport.y * clipPos.w * 2.0;
  PointsOut out;
  out.clipPos = float4(clipPos.x + ndcX, clipPos.y + ndcY, clipPos.z, clipPos.w);
  out.v_color = (pts.flags.x != 0u) ? p.color.rgb : pts.defaultColorAndSize.rgb;
  out.v_alpha = p.color.a * pts.params.x;
  out.v_pointCoord = quad + float2(0.5, 0.5);
  out.v_pointSize = pixelSize;
  if (pts.params.y != 0.0) {
    float dist = length(viewPos.xyz);
    out.v_fogFactor = clamp(exp(-pts.params.z * pts.params.z * dist * dist), 0.0, 1.0);
  } else {
    out.v_fogFactor = 1.0;
  }
  return out;
}

fragment float4 fragmentMain(PointsOut in [[stage_in]], constant UserUniforms& u [[buffer(1)]]) {
  float2 centered = (in.v_pointCoord - float2(0.5, 0.5));
  float radial = (length(centered) * 2.0);
  float sizeFocus = clamp(((in.v_pointSize - 4.0) / 48.0), 0.0, 1.0);
  float falloff = mix(4.2, 3.2, sizeFocus);
  float core = exp((-((radial * radial) * falloff)));
  float edge = (1.0 - smoothstep(0.78, 1.0, radial));
  float a = ((core * edge) * in.v_alpha);
  float3 foggedRGB = mix(u.fogColor, in.v_color, in.v_fogFactor);
  return float4(foggedRGB.r, foggedRGB.g, foggedRGB.b, a);
}
