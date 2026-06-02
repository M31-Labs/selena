struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  baseColor : vec3<f32>,
  light_ambient : f32,
  light_dir : vec3<f32>,
  phase : f32,
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
  let n = normalize(in.vWorldNormal);
  let diffuse = max(dot(n, u.light_dir), 0.0);
  let ripple = (sin(((in.vUv.x * 12.0) + u.phase)) * cos((in.vUv.y * 12.0)));
  let bands = step(0.5, fract((in.vUv.y * 6.0)));
  let glow = smoothstep(0.1, 0.9, fract((in.vUv.x * 4.0)));
  let tangent = cross(n, u.light_dir);
  let rim = pow((1.0 - diffuse), 3.0);
  let bounce = reflect(u.light_dir, n);
  let shade = (((((u.light_ambient + diffuse) + (ripple * 0.15)) + (bands * 0.1)) + (glow * 0.2)) + (rim * 0.25));
  return vec4<f32>((((u.baseColor * shade) + (sqrt(abs(tangent.x)) * 0.05)) + (bounce.y * 0.02)), 1.0);
}
