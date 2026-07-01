struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  poolWidth : f32,
  poolLength : f32,
  poolHeight : f32,
  lightDir : vec3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

@group(0) @binding(1) var tileTexture : texture_2d<f32>;
@group(0) @binding(2) var tileTextureSampler : sampler;
@group(0) @binding(3) var causticTexture : texture_2d<f32>;
@group(0) @binding(4) var causticTextureSampler : sampler;
@group(0) @binding(5) var shadowTexture : texture_2d<f32>;
@group(0) @binding(6) var shadowTextureSampler : sampler;

struct StateGrid {
  gridWidth  : u32,
  gridHeight : u32,
};
@group(0) @binding(7) var<uniform> _stateGrid : StateGrid;
@group(0) @binding(8) var<storage, read> _inState : array<vec4<f32>>;

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vWorldPos : vec3<f32>,
  @location(1) vNormal : vec3<f32>,
  @location(2) vTileUV : vec2<f32>,
  @location(3) vWaterUV : vec2<f32>,
  @location(4) vFace : f32,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex : u32) -> VertexOutput {
  var out : VertexOutput;
  let fi = f32(vertexIndex);
  let fv = floor((fi / 6.0));
  let faceF = min(fv, 4.0);
  let cu = (fi - (fv * 6.0));
  let uf = select(select(select(0.0, 1.0, (cu == 4.0)), 1.0, (cu == 2.0)), 1.0, (cu == 1.0));
  let vf = select(select(select(0.0, 1.0, (cu == 5.0)), 1.0, (cu == 4.0)), 1.0, (cu == 2.0));
  let hw = max(u.poolWidth, 0.001);
  let hl = max(u.poolLength, 0.001);
  let flY = (-max(u.poolHeight, 0.001));
  let rimY = max((u.poolHeight * 0.1667), 0.025);
  let wx = select(select(select(mix((-hw), hw, uf), (-hw), (faceF == 4.0)), hw, (faceF == 3.0)), mix(hw, (-hw), uf), (faceF == 2.0));
  let wy = select(mix(flY, rimY, vf), flY, (faceF == 0.0));
  let wz = select(select(select(select(mix((-hl), hl, vf), mix((-hl), hl, uf), (faceF == 4.0)), mix(hl, (-hl), uf), (faceF == 3.0)), (-hl), (faceF == 2.0)), hl, (faceF == 1.0));
  let nx = select(select(0.0, 1.0, (faceF == 4.0)), (-1.0), (faceF == 3.0));
  let ny = select(0.0, 1.0, (faceF == 0.0));
  let nz = select(select(0.0, 1.0, (faceF == 2.0)), (-1.0), (faceF == 1.0));
  let tileX = select(select((wx * 0.42), (wz * 0.42), (faceF == 4.0)), (wz * 0.42), (faceF == 3.0));
  let tileY = select((wy * 0.72), (wz * 0.42), (faceF == 0.0));
  let duw = max((u.poolWidth * 2.0), 0.001);
  let dul = max((u.poolLength * 2.0), 0.001);
  out.vWorldPos = vec3<f32>(wx, wy, wz);
  out.vNormal = vec3<f32>(nx, ny, nz);
  out.vTileUV = vec2<f32>(tileX, tileY);
  out.vWaterUV = vec2<f32>(((wx / duw) + 0.5), ((wz / dul) + 0.5));
  out.vFace = faceF;
  out.position = (u.mvp * vec4<f32>(wx, wy, wz, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let wuv = clamp(in.vWaterUV, vec2<f32>(0.0, 0.0), vec2<f32>(1.0, 1.0));
  let info = _inState[min(u32((wuv).x * f32(_stateGrid.gridWidth)) + u32((wuv).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let wh = (info.x * u.poolHeight);
  let ldir = normalize(u.lightDir);
  let refr = refract((-ldir), vec3<f32>(0.0, 1.0, 0.0), (1.0 / 1.333));
  let refY = select(0.05, refr.y, (abs(refr.y) > 0.05));
  let duw = max((u.poolWidth * 2.0), 0.001);
  let dul = max((u.poolLength * 2.0), 0.001);
  let projX = ((in.vWorldPos.x - ((in.vWorldPos.y * refr.x) / refY)) / duw);
  let projZ = ((in.vWorldPos.z - ((in.vWorldPos.y * refr.z) / refY)) / dul);
  let cUV = clamp(vec2<f32>(((projX * 0.75) + 0.5), ((projZ * 0.75) + 0.5)), vec2<f32>(0.0, 0.0), vec2<f32>(1.0, 1.0));
  let tileSamp = textureSample(tileTexture, tileTextureSampler, in.vTileUV);
  let causticS = textureSample(causticTexture, causticTextureSampler, cUV);
  let shadowS = textureSample(shadowTexture, shadowTextureSampler, wuv);
  let tileRGB = tileSamp.xyz;
  let caustic = causticS.xyz;
  let shadowV = shadowS.x;
  let nrm = normalize(in.vNormal);
  let diffuse = max(dot(nrm, normalize((-refr))), 0.0);
  let below = select(0.0, 1.0, (in.vWorldPos.y < wh));
  let distFade = (1.0 / max((length(in.vWorldPos) * 0.52), 1.0));
  let dryLight = (0.46 + (diffuse * 0.34));
  let causticE = dot(caustic, vec3<f32>(0.34, 0.44, 0.22));
  let base = ((tileRGB * dryLight) * distFade);
  let wet = (((base * vec3<f32>(0.42, 0.92, 1.0)) * (0.72 + (diffuse * 0.22))) + (caustic * (1.55 + (causticE * 0.6))));
  let col0 = mix(base, wet, below);
  let col1 = (col0 * (1.0 - (clamp(shadowV, 0.0, 1.0) * 0.62)));
  let rim = smoothstep(0.0, 0.12, in.vWorldPos.y);
  let rimAdd = vec3<f32>(0.05, 0.035, 0.018);
  let col2 = mix(col1, (col1 + rimAdd), (rim * (1.0 - below)));
  return vec4<f32>(vec3<f32>(col2.x, col2.y, col2.z), 1.0);
}
