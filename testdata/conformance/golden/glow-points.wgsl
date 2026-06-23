struct FrameUniforms {
  viewMatrix     : mat4x4<f32>,
  projMatrix     : mat4x4<f32>,
  viewportWidth  : f32,
  viewportHeight : f32,
};

struct PointsUniforms {
  modelMatrix        : mat4x4<f32>,
  defaultColorAndSize: vec4<f32>,
  flags              : vec4<u32>,
  params             : vec4<f32>,
  fogColor           : vec4<f32>,
};

struct ParticleInstance {
  position : vec3<f32>,
  size     : f32,
  color    : vec4<f32>,
};

@group(0) @binding(0) var<uniform> _frame     : FrameUniforms;
@group(2) @binding(0) var<uniform> _points    : PointsUniforms;
@group(2) @binding(1) var<storage, read> _particles : array<ParticleInstance>;

struct UserUniforms {
  fogColor : vec3<f32>,
};
@group(1) @binding(0) var<uniform> u : UserUniforms;

struct PointsOutput {
  @builtin(position) clipPos : vec4<f32>,
  @location(0) v_color       : vec3<f32>,
  @location(1) v_fogFactor   : f32,
  @location(2) v_alpha       : f32,
  @location(3) v_pointCoord  : vec2<f32>,
  @location(4) v_pointSize   : f32,
};

// Attribute input for the static-layer vertex entry (vertexMain).
struct PointsAttribInput {
  @location(0) a_position : vec3<f32>,
  @location(1) a_size     : f32,
  @location(2) a_color    : vec4<f32>,
};

const _quadPos = array<vec2<f32>, 6>(
  vec2<f32>(-0.5, -0.5), vec2<f32>(0.5, -0.5), vec2<f32>(-0.5, 0.5),
  vec2<f32>(0.5, -0.5),  vec2<f32>(0.5, 0.5),  vec2<f32>(-0.5, 0.5),
);

@vertex fn vertexMain(
  @builtin(vertex_index) vertexIndex : u32,
  in : PointsAttribInput,
) -> PointsOutput {
  let quad = _quadPos[vertexIndex];

  let worldPos = (_points.modelMatrix * vec4<f32>(in.a_position, 1.0)).xyz;
  let viewPos  = _frame.viewMatrix * vec4<f32>(worldPos, 1.0);

  var rawSize : f32;
  if (_points.flags.y == 0u) { rawSize = _points.defaultColorAndSize.w; }
  else { rawSize = in.a_size; }

  var pixelSize : f32;
  if (_points.flags.z != 0u) {
    pixelSize = max(rawSize * (_frame.viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(rawSize, 1.0);
  }
  let _minPx = max(_points.fogColor.a, 0.0);
  if (_minPx > 0.0) { pixelSize = max(pixelSize, _minPx); }
  if (_points.params.w > 0.0) { pixelSize = min(pixelSize, _points.params.w); }

  let clipPos    = _frame.projMatrix * viewPos;
  let ndcOffsetX = quad.x * pixelSize / _frame.viewportWidth  * clipPos.w * 2.0;
  let ndcOffsetY = quad.y * pixelSize / _frame.viewportHeight * clipPos.w * 2.0;

  var out : PointsOutput;
  out.clipPos = vec4<f32>(clipPos.x + ndcOffsetX, clipPos.y + ndcOffsetY, clipPos.z, clipPos.w);

  if (_points.flags.x != 0u) { out.v_color = in.a_color.rgb; }
  else { out.v_color = _points.defaultColorAndSize.rgb; }
  out.v_alpha      = in.a_color.a * _points.params.x;
  out.v_pointCoord = quad + vec2<f32>(0.5, 0.5);
  out.v_pointSize  = pixelSize;

  if (_points.params.y != 0.0) {
    let dist = length(viewPos.xyz);
    out.v_fogFactor = clamp(exp(-_points.params.z * _points.params.z * dist * dist), 0.0, 1.0);
  } else {
    out.v_fogFactor = 1.0;
  }
  return out;
}

@vertex fn vertexStorageMain(
  @builtin(vertex_index)   vertexIndex   : u32,
  @builtin(instance_index) instanceIndex : u32,
) -> PointsOutput {
  let quad = _quadPos[vertexIndex];
  let p    = _particles[instanceIndex];

  let worldPos = (_points.modelMatrix * vec4<f32>(p.position, 1.0)).xyz;
  let viewPos  = _frame.viewMatrix * vec4<f32>(worldPos, 1.0);

  var rawSize : f32;
  if (_points.flags.y == 0u) { rawSize = _points.defaultColorAndSize.w; }
  else { rawSize = p.size; }

  var pixelSize : f32;
  if (_points.flags.z != 0u) {
    pixelSize = max(rawSize * (_frame.viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
  } else {
    pixelSize = max(rawSize, 1.0);
  }
  let _minPx = max(_points.fogColor.a, 0.0);
  if (_minPx > 0.0) { pixelSize = max(pixelSize, _minPx); }
  if (_points.params.w > 0.0) { pixelSize = min(pixelSize, _points.params.w); }

  let clipPos    = _frame.projMatrix * viewPos;
  let ndcOffsetX = quad.x * pixelSize / _frame.viewportWidth  * clipPos.w * 2.0;
  let ndcOffsetY = quad.y * pixelSize / _frame.viewportHeight * clipPos.w * 2.0;

  var out : PointsOutput;
  out.clipPos = vec4<f32>(clipPos.x + ndcOffsetX, clipPos.y + ndcOffsetY, clipPos.z, clipPos.w);

  if (_points.flags.x != 0u) { out.v_color = p.color.rgb; }
  else { out.v_color = _points.defaultColorAndSize.rgb; }
  out.v_alpha      = p.color.a * _points.params.x;
  out.v_pointCoord = quad + vec2<f32>(0.5, 0.5);
  out.v_pointSize  = pixelSize;

  if (_points.params.y != 0.0) {
    let dist = length(viewPos.xyz);
    out.v_fogFactor = clamp(exp(-_points.params.z * _points.params.z * dist * dist), 0.0, 1.0);
  } else {
    out.v_fogFactor = 1.0;
  }
  return out;
}

struct PointsInput {
  @location(0) v_color      : vec3<f32>,
  @location(1) v_fogFactor  : f32,
  @location(2) v_alpha      : f32,
  @location(3) v_pointCoord : vec2<f32>,
  @location(4) v_pointSize  : f32,
};

@fragment fn fragmentMain(in : PointsInput) -> @location(0) vec4<f32> {
  let centered = (in.v_pointCoord - vec2<f32>(0.5, 0.5));
  let radial = (length(centered) * 2.0);
  let sizeFocus = clamp(((in.v_pointSize - 4.0) / 48.0), 0.0, 1.0);
  let falloff = mix(4.2, 3.2, sizeFocus);
  let core = exp((-((radial * radial) * falloff)));
  let edge = (1.0 - smoothstep(0.78, 1.0, radial));
  let a = ((core * edge) * in.v_alpha);
  let foggedRGB = mix(u.fogColor, in.v_color, in.v_fogFactor);
  return vec4<f32>(foggedRGB.r, foggedRGB.g, foggedRGB.b, a);
}
