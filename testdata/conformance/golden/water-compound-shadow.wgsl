@group(0) @binding(0) var _sceneColorTex  : texture_2d<f32>;
@group(0) @binding(1) var _sceneColorSamp : sampler;
@group(0) @binding(2) var _sceneDepthTex  : texture_depth_2d;
@group(0) @binding(3) var _sceneDepthSamp : sampler;

struct UserUniforms {
  sphereCount : f32,
  objectEnabled : f32,
  objectTop : f32,
  lightDir : vec3<f32>,
  poolWidth : f32,
  poolLength : f32,
  objectCenterX : f32,
  objectCenterZ : f32,
  spheres : array<vec4<f32>, 32>,
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
  let aspect = vec2<f32>(max((u.poolWidth / max(u.poolLength, 0.001)), 0.001), 1.0);
  var mask : f32 = 0.0;
  var core : f32 = 0.0;
  for (var i : i32 = 0i; (i < 32i); i = (i + 1i)) {
    if (((u.objectEnabled > 0.0) && (f32(i) < u.sphereCount))) {
      let sphere = u.spheres[i];
      let sphereUVx = (((u.objectCenterX + sphere.x) * 0.5) + 0.5);
      let sphereUVy = (((u.objectCenterZ + sphere.z) * 0.5) + 0.5);
      let radius = max((sphere.w * 0.58), 0.012);
      let dvec = vec2<f32>(((uvX - sphereUVx) * aspect.x), ((uvY - sphereUVy) * aspect.y));
      let dd = length(dvec);
      let localMask = (1.0 - smoothstep(radius, (radius + max((radius * 1.35), 0.018)), dd));
      mask = max(mask, localMask);
      core = max(core, (1.0 - smoothstep((radius * 0.42), radius, dd)));
    }
  }
  let clipped = smoothstep((-0.08), 0.16, u.objectTop);
  let shadow = ((mask * (0.42 + (0.58 * core))) * clipped);
  return vec4<f32>(vec3<f32>(shadow, shadow, shadow), 1.0);
}
