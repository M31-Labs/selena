struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  limit : f32,
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
  var hits : f32 = 0.0;
  for (var i : f32 = 0.0; (i < 8.0); i = (i + 1.0)) {
    if ((i > u.limit)) {
      return vec4<f32>(vec3<f32>(1.0, 0.0, 0.0), 1.0);
    }
    hits = (hits + 1.0);
    if ((hits > 6.0)) {
      break;
    }
  }
  return vec4<f32>(vec3<f32>(0.0, (hits / 8.0), 0.0), 1.0);
}
