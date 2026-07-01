#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float a;
  float b;
};

struct VertexIn {
  float3 position [[attribute(0)]];
};

struct VertexOut {
  float4 position [[position]];
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  VertexOut out;
  out.position = (u.mvp * float4(in.position, 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {
  bool lt = (u.a < u.b);
  bool gt = (u.a > u.b);
  bool le = (u.a <= u.b);
  bool ge = (u.a >= u.b);
  bool eq = (u.a == u.b);
  bool ne = (u.a != u.b);
  bool both = (lt && gt);
  bool any = (le || ge);
  bool notv = (!eq);
  float sel = (ne ? u.a : u.b);
  return float4(float3(sel, 0.0, 0.0), 1.0);
}
