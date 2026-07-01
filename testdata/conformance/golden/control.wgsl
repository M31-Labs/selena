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
  var n : i32 = 4i;
  var acc : i32 = 0i;
  for (var i : i32 = 0i; (i < n); i = (i + 1i)) {
    acc = (acc + 1i);
  }
  var r : f32 = 0.0;
  var g : f32 = 0.0;
  if ((acc == n)) {
    r = u.baseColor.r;
  } else {
    g = u.baseColor.g;
  }
  var result : f32 = r;
  result = (result + g);
  return vec4<f32>(vec3<f32>(result, u.baseColor.g, u.baseColor.b), 1.0);
}
