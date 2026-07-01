struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  gridSize : f32,
  amplitude : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) worldPos : vec3<f32>,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex : u32) -> VertexOutput {
  var out : VertexOutput;
  let fi = f32(vertexIndex);
  let col = fract((fi / u.gridSize));
  let row = (floor((fi / u.gridSize)) / u.gridSize);
  let x = ((col * 2.0) - 1.0);
  let z = ((row * 2.0) - 1.0);
  var y : f32 = 0.0;
  var n : i32 = 3i;
  for (var i : i32 = 0i; (i < n); i = (i + 1i)) {
    let phase = (f32(i) * 1.5707964);
    y = (y + (sin(((x * 6.2831855) + phase)) * u.amplitude));
  }
  if ((y > u.amplitude)) {
    y = u.amplitude;
  }
  let p = vec3<f32>(x, y, z);
  out.worldPos = p;
  out.position = (u.mvp * vec4<f32>(x, y, z, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let h = ((in.worldPos.y * 0.5) + 0.5);
  return vec4<f32>(vec3<f32>(h, ((in.worldPos.x * 0.5) + 0.5), ((in.worldPos.z * 0.5) + 0.5)), 1.0);
}
