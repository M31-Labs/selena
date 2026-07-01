struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
  gain : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vWorldNormal : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let n = normalize(in.vWorldNormal);
  let white = vec3<f32>(1.0, 1.0, 1.0);
  let lit = max(n.y, 0.0);
  var acc : f32 = 0.0;
  if ((lit > 0.5)) {
    acc = u.gain;
  } else {
    acc = (u.gain * 0.5);
  }
  for (var i : i32 = 0i; (i < 2i); i = (i + 1i)) {
    acc = (acc + 0.1);
  }
  return vec4<f32>(((u.baseColor * (acc * lit)) + (white * 0.0)), 1.0);
}
