#include <metal_stdlib>
using namespace metal;

struct Uniforms {
  float4x4 mvp;
  float3x3 normalMatrix;
  float head;
};

struct VertexIn {
  float3 position [[attribute(0)]];
  float3 normal [[attribute(1)]];
  float2 uv [[attribute(2)]];
};

struct VertexOut {
  float4 position [[position]];
  float vLevel;
  float2 vUv;
};

vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> gains [[texture(1)]], sampler gainsSampler [[sampler(1)]]) {
  VertexOut out;
  float columns = float2(gains.get_width(), gains.get_height()).x;
  int column = int(clamp((in.uv.x * columns), 0.0, (columns - 1.0)));
  float level = gains.read(uint2(column, 0), 0).r;
  out.vLevel = level;
  out.vUv = in.uv;
  out.position = (u.mvp * float4((in.position + (in.normal * level)), 1.0));
  return out;
}

fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]], texture2d<float> spectrum [[texture(0)]], sampler spectrumSampler [[sampler(0)]], texture2d<float> gains [[texture(1)]], sampler gainsSampler [[sampler(1)]], texture2d<float> palette [[texture(2)]], sampler paletteSampler [[sampler(2)]]) {
  float2 sz = float2(spectrum.get_width(), spectrum.get_height());
  int col = int((((in.vUv.x * sz.x) + u.head) - (sz.x * floor(((in.vUv.x * sz.x) + u.head) / sz.x))));
  int row = int((in.vUv.y * sz.y));
  float mag = spectrum.read(uint2(col, row), 0).r;
  float4 exact = palette.read(uint2(0, 0), 0);
  float4 soft = palette.sample(paletteSampler, in.vUv);
  return float4(float3(((mag * exact.r) + (soft.r * 0.25)), ((mag * 0.6) + in.vLevel), (1.0 - mag)), 1.0);
}
