struct Uniforms {
  strips : f32,
  screenSize : vec2<f32>,
  levels : array<vec4<f32>, 64>,
  holds : array<vec4<f32>, 64>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct ScreenOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) v_uv           : vec2<f32>,
};

const _screenPositions = array<vec2<f32>, 3>(
  vec2<f32>(-1.0, -1.0),
  vec2<f32>( 3.0, -1.0),
  vec2<f32>(-1.0,  3.0),
);
const _screenUVs = array<vec2<f32>, 3>(
  vec2<f32>(0.0, 1.0),
  vec2<f32>(2.0, 1.0),
  vec2<f32>(0.0, -1.0),
);

@vertex fn vertexMain(@builtin(vertex_index) vi : u32) -> ScreenOutput {
  var out : ScreenOutput;
  out.position = vec4<f32>(_screenPositions[vi], 0.0, 1.0);
  out.v_uv     = _screenUVs[vi];
  return out;
}

struct ScreenInput {
  @location(0) v_uv : vec2<f32>,
};

@fragment fn fragmentMain(in : ScreenInput) -> @location(0) vec4<f32> {
  let x = (in.v_uv.x * u.strips);
  let i = i32(min(floor(x), 63.0));
  let lane = fract(x);
  let gap = (u.strips / u.screenSize.x);
  let body = (step(gap, lane) * step(lane, (1.0 - gap)));
  let l = u.levels[i];
  let h = u.holds[i];
  let peak = select(l.y, l.x, (lane < 0.5));
  let hold = select(h.y, h.x, (lane < 0.5));
  let up = (1.0 - in.v_uv.y);
  let bar = step(up, ((peak + 60.0) / 60.0));
  let mark = (step(abs((up - ((hold + 60.0) / 60.0))), gap) * 0.5);
  let lit = (max(bar, mark) * body);
  return vec4<f32>((lit * 0.2), lit, (lit * 0.4), lit);
}
