@group(0) @binding(0) var _sceneColorTex  : texture_2d<f32>;
@group(0) @binding(1) var _sceneColorSamp : sampler;
@group(0) @binding(2) var _sceneDepthTex  : texture_depth_2d;
@group(0) @binding(3) var _sceneDepthSamp : sampler;

struct UserUniforms {
  lensCenterX : f32,
  lensCenterY : f32,
  strength : f32,
  softening : f32,
  maxBend : f32,
};
@group(0) @binding(4) var<uniform> u : UserUniforms;

struct PostOutput {
  @builtin(position) pos : vec4<f32>,
  @location(0) v_uv      : vec2<f32>,
};

const _postPositions = array<vec2<f32>, 3>(
  vec2<f32>(-1.0, -1.0),
  vec2<f32>( 3.0, -1.0),
  vec2<f32>(-1.0,  3.0),
);
const _postUVs = array<vec2<f32>, 3>(
  vec2<f32>(0.0, 1.0),
  vec2<f32>(2.0, 1.0),
  vec2<f32>(0.0, -1.0),
);

@vertex fn vertexMain(@builtin(vertex_index) vi : u32) -> PostOutput {
  var out : PostOutput;
  out.pos  = vec4<f32>(_postPositions[vi], 0.0, 1.0);
  out.v_uv = _postUVs[vi];
  return out;
}

struct PostInput {
  @location(0) v_uv : vec2<f32>,
};

@fragment fn fragmentMain(in : PostInput) -> @location(0) vec4<f32> {
  let lensCenter = vec2<f32>(u.lensCenterX, u.lensCenterY);
  let delta = (in.v_uv - lensCenter);
  let r2 = (dot(delta, delta) + (u.softening * u.softening));
  let bend = (u.strength / r2);
  let bendClamped = clamp(bend, 0.0, u.maxBend);
  let disp = (delta * bendClamped);
  let dispR = (disp * 1.15);
  let dispB = (disp * 0.85);
  let colorR = textureSample(_sceneColorTex, _sceneColorSamp, (in.v_uv - dispR));
  let colorG = textureSample(_sceneColorTex, _sceneColorSamp, (in.v_uv - disp));
  let colorB = textureSample(_sceneColorTex, _sceneColorSamp, (in.v_uv - dispB));
  let lensed = vec4<f32>(colorR.r, colorG.g, colorB.b, 1.0);
  let rawColor = textureSample(_sceneColorTex, _sceneColorSamp, in.v_uv);
  let distCenter = length(delta);
  let blendEdge = smoothstep(0.0, (u.softening * 4.0), distCenter);
  return mix(rawColor, lensed, blendEdge);
}
