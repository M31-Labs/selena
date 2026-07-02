struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  poolHeight : f32,
  baseColor : vec4<f32>,
  isTexturePass : f32,
  texturePassMode : f32,
  lightDir : vec3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var modelTexture : texture_2d<f32>;
@group(0) @binding(2) var modelTextureSampler : sampler;

struct StateGrid {
  gridWidth  : u32,
  gridHeight : u32,
};
@group(0) @binding(3) var<uniform> _stateGrid : StateGrid;
@group(0) @binding(4) var<storage, read> _inState : array<vec4<f32>>;

struct VertexInput {
  @location(0) position : vec3<f32>,
  @location(1) normal : vec3<f32>,
  @location(2) uv : vec2<f32>,
};

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) worldPos : vec3<f32>,
  @location(1) vUv : vec2<f32>,
  @location(2) vNormal : vec3<f32>,
};

@vertex
fn vertexMain(in : VertexInput) -> VertexOutput {
  var out : VertexOutput;
  out.worldPos = in.position;
  out.vUv = in.uv;
  out.vNormal = normalize((u.normalMatrix * in.normal));
  out.position = (u.mvp * vec4<f32>(in.position, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let info = _inState[min(u32((in.vUv).x * f32(_stateGrid.gridWidth)) + u32((in.vUv).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let waterHeight = (info.x * u.poolHeight);
  let caustic = info.y;
  let belowWater = (in.worldPos.y < waterHeight);
  if ((((u.isTexturePass > 0.5) && (u.texturePassMode == 2.0)) && belowWater)) {
    discard;
  }
  let lightN = normalize(u.lightDir);
  let refr = refract((-lightN), vec3<f32>(0.0, 1.0, 0.0), (1.0 / 1.333));
  let n = normalize(in.vNormal);
  let albedo = (textureSample(modelTexture, modelTextureSampler, in.vUv).rgb * u.baseColor.rgb);
  let submerged = (in.worldPos.y < waterHeight);
  var diffuse : f32 = (max(dot((-refr), n), 0.0) * 0.6);
  if (submerged) {
    diffuse = ((diffuse * caustic) * 4.0);
  }
  let underwater = vec3<f32>(0.4, 0.9, 1.0);
  var col : vec3<f32> = (albedo * (0.4 + diffuse));
  if (submerged) {
    col = ((col * underwater) * 1.2);
  }
  return vec4<f32>(col, u.baseColor.a);
}
