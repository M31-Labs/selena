#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float3 baseColor;
  float light_ambient;
  float3 light_dir;
  float phase;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
  float2 uv [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float2 vUv;
  float3 vWorldNormal;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.vUv = in.uv;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  float3 n = normalize(in.vWorldNormal);
  float diffuse = max(dot(n, u.light_dir), 0.0);
  float ripple = (sin(((in.vUv.x * 12.0) + u.phase)) * cos((in.vUv.y * 12.0)));
  float bands = step(0.5, fract((in.vUv.y * 6.0)));
  float glow = smoothstep(0.1, 0.9, fract((in.vUv.x * 4.0)));
  float3 tangent = cross(n, u.light_dir);
  float rim = pow((1.0 - diffuse), 3.0);
  float3 bounce = reflect(u.light_dir, n);
  float shade = (((((u.light_ambient + diffuse) + (ripple * 0.15)) + (bands * 0.1)) + (glow * 0.2)) + (rim * 0.25));
  return float4((((u.baseColor * shade) + (sqrt(abs(tangent.x)) * 0.05)) + (bounce.y * 0.02)), 1.0);
}
