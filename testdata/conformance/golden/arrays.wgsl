struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
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
  var a : array<f32, 3>;
  for (var i : i32 = 0i; (i < 3i); i = (i + 1i)) {
    a[i] = 1.0;
  }
  a[0i] = u.baseColor.r;
  a[1i] = u.baseColor.g;
  a[2i] = u.baseColor.b;
  let r = a[0i];
  let g = a[1i];
  let b = a[2i];
  return vec4<f32>(vec3<f32>(r, g, b), 1.0);
}
