struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  lightDir : vec3<f32>,
  poolHalfW : f32,
  poolHalfL : f32,
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
  let pos = in.position;
  let lnorm = normalize(u.lightDir);
  let refr = refract((-lnorm), vec3<f32>(0.0, 1.0, 0.0), (1.0 / 1.333));
  let fallY = select((-0.0001), 0.0001, (refr.y >= 0.0));
  let refY = select(fallY, refr.y, (abs(refr.y) > 0.0001));
  let pX = (0.75 * (pos.x - ((pos.y * refr.x) / refY)));
  let pZ = (0.75 * (pos.z - ((pos.y * refr.z) / refY)));
  let clipX = (pX / max(u.poolHalfW, 0.0001));
  let clipZ = (pZ / max(u.poolHalfL, 0.0001));
  out.position = vec4<f32>(clipX, clipZ, 0.0, 1.0);
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  return vec4<f32>(vec3<f32>(1.0, 1.0, 1.0), 1.0);
}
