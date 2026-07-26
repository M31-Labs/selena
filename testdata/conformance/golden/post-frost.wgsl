@group(0) @binding(0) var _sceneColorTex  : texture_2d<f32>;
@group(0) @binding(1) var _sceneColorSamp : sampler;
@group(0) @binding(2) var _sceneDepthTex  : texture_depth_2d;
@group(0) @binding(3) var _sceneDepthSamp : sampler;

struct UserUniforms {
  blurLevel : f32,
  mixAmount : f32,
  rects : array<vec4<f32>, 4>,
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
  let size = vec2<f32>(textureDimensions(_sceneColorTex, 0));
  let px = (1.0 / max(size.x, 1.0));
  var best : f32 = 1.0;
  for (var i : i32 = 0i; (i < 4i); i = (i + 1i)) {
    let r = u.rects[i];
    let dx = (abs((in.v_uv.x - r.x)) - r.z);
    let dy = (abs((in.v_uv.y - r.y)) - r.w);
    let d = (length(vec2<f32>(max(dx, 0.0), max(dy, 0.0))) + min(max(dx, dy), 0.0));
    best = min(best, d);
    if ((d < 0.0)) {
      break;
    }
  }
  let aa = max(fwidth(best), px);
  let inside = (1.0 - smoothstep((0.0 - aa), (0.0 + aa), best));
  let blurred = textureSampleLevel(_sceneColorTex, _sceneColorSamp, in.v_uv, u.blurLevel);
  let plain = textureSample(_sceneColorTex, _sceneColorSamp, in.v_uv);
  let k = (inside * u.mixAmount);
  return vec4<f32>(mix(plain.r, blurred.r, k), mix(plain.g, blurred.g, k), mix(plain.b, blurred.b, k), plain.a);
}
