@group(0) @binding(0) var _sceneColorTex  : texture_2d<f32>;
@group(0) @binding(1) var _sceneColorSamp : sampler;
@group(0) @binding(2) var _sceneDepthTex  : texture_depth_2d;
@group(0) @binding(3) var _sceneDepthSamp : sampler;

struct UserUniforms {
  objectKind : f32,
  objectEnabled : f32,
  lightDir : vec3<f32>,
  poolWidth : f32,
  poolLength : f32,
  objectCenterX : f32,
  objectCenterZ : f32,
  objectRadius : f32,
  objectHalfX : f32,
  objectHalfZ : f32,
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
  let lnorm = normalize(u.lightDir);
  let uvX = clamp((in.v_uv.x - (lnorm.x * 0.025)), 0.0, 1.0);
  let uvY = clamp((in.v_uv.y + (lnorm.z * 0.025)), 0.0, 1.0);
  let centerUV = vec2<f32>(((u.objectCenterX * 0.5) + 0.5), ((u.objectCenterZ * 0.5) + 0.5));
  let aspect = vec2<f32>(max((u.poolWidth / max(u.poolLength, 0.001)), 0.001), 1.0);
  let dd = length((vec2<f32>((uvX - centerUV.x), (uvY - centerUV.y)) * aspect));
  let sphR = max((u.objectRadius * 0.55), 0.018);
  let cubeR = max((max(u.objectHalfX, u.objectHalfZ) * 0.6), sphR);
  let radius = select(sphR, cubeR, (u.objectKind > 1.5));
  var mask : f32 = 0.0;
  var core : f32 = 0.0;
  if (((u.objectKind >= 0.5) && (u.objectEnabled > 0.0))) {
    mask = (1.0 - smoothstep(radius, (radius + max((radius * 1.2), 0.02)), dd));
    core = (1.0 - smoothstep((radius * 0.38), radius, dd));
  }
  let shadow = (mask * (0.42 + (core * 0.58)));
  return vec4<f32>(vec3<f32>(shadow, shadow, shadow), 1.0);
}
