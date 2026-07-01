struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  tint : vec4<f32>,
  items : array<vec4<f32>, 8>,
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
  var acc : vec4<f32> = u.tint;
  for (var i : i32 = 0i; (i < 8i); i = (i + 1i)) {
    let item = u.items[i];
    acc = (acc + item);
  }
  return vec4<f32>(acc.rgb, 1.0);
}
