struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  a : f32,
  b : f32,
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
  let lt = (u.a < u.b);
  let gt = (u.a > u.b);
  let le = (u.a <= u.b);
  let ge = (u.a >= u.b);
  let eq = (u.a == u.b);
  let ne = (u.a != u.b);
  let both = (lt && gt);
  let any = (le || ge);
  let notv = (!eq);
  let sel = select(u.b, u.a, ne);
  return vec4<f32>(vec3<f32>(sel, 0.0, 0.0), 1.0);
}
