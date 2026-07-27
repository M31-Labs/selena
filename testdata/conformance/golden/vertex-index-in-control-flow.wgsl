struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  gridSize : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) shade : f32,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex : u32) -> VertexOutput {
  var out : VertexOutput;
  var fi : f32 = 0.0;
  if ((u.gridSize > 0.0)) {
    fi = f32(vertexIndex);
  }
  let col = fract((fi / u.gridSize));
  let row = (floor((fi / u.gridSize)) / u.gridSize);
  out.shade = col;
  out.position = (u.mvp * vec4<f32>(((col * 2.0) - 1.0), 0.0, ((row * 2.0) - 1.0), 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  return vec4<f32>(vec3<f32>(in.shade, in.shade, in.shade), 1.0);
}
