struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  ior : f32,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vUv : vec2<f32>,
  @location(1) vWorldNormal : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.vUv = in.uv;
  out.vWorldNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let st = in.vUv;
  let n = normalize(in.vWorldNormal);
  let ri = refract(n, n, u.ior);
  let m = (st.x - (0.5 * floor(st.x / 0.5)));
  let rnd = round(st.y);
  let a1 = atan(st.x);
  let a2 = atan2(st.y, st.x);
  let ai = asin(m);
  let ac = acos(m);
  let dx = dpdx(st.x);
  let dy = dpdy(st.y);
  let fw = fwidth(m);
  let v3 = vec3<f32>(m, rnd, a1);
  let v4 = vec4<f32>(st, ai, ac);
  return vec4<f32>(((ri + (v3 * (((a2 + dx) + dy) + fw))) + v4.xyz), 1.0);
}
