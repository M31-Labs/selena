struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
  lo : f32,
  hi : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let clamped = clamp(u.baseColor, vec3<f32>(u.lo), vec3<f32>(u.hi));
  let bright = max(u.baseColor, vec3<f32>(u.lo));
  let dark = min(u.baseColor, vec3<f32>(u.hi));
  let blended = mix(u.baseColor, clamped, u.lo);
  let stepped = step(vec3<f32>(u.lo), u.baseColor);
  let powered = pow(clamped, vec3<f32>(u.hi));
  return vec4<f32>((((((clamped + (bright * 0.0)) + (dark * 0.0)) + (blended * 0.0)) + (stepped * 0.0)) + (powered * 0.0)), 1.0);
}
