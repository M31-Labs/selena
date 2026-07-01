struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  poolWidth : f32,
  poolLength : f32,
  poolHeight : f32,
  cornerRadius : f32,
  poolShape : f32,
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
  let hw = max(u.poolWidth, 0.001);
  let hl = max(u.poolLength, 0.001);
  let flY = (-max(u.poolHeight, 0.001));
  let rimY = max((u.poolHeight * 0.1667), 0.025);
  let maxCornerRadius = max(0.0, (min(hw, hl) - 0.001));
  let cr = clamp(u.cornerRadius, 0.0, maxCornerRadius);
  let roundedActive = ((u.poolShape > 0.5) && (cr > 0.0001));
  var wx : f32 = 0.0;
  var wy : f32 = 0.0;
  var wz : f32 = 0.0;
  var nx : f32 = 0.0;
  var ny : f32 = 1.0;
  var nz : f32 = 0.0;
  var tileX : f32 = 0.0;
  var tileY : f32 = 0.0;
  var faceOut : f32 = 0.0;
  if (roundedActive) {
    let insetX = max((hw - cr), 0.001);
    let insetY = max((hl - cr), 0.001);
    if ((fi < 132.0)) {
      let triF = floor((fi / 3.0));
      let triCorner = (fi - (triF * 3.0));
      var px : f32 = 0.0;
      var py : f32 = 0.0;
      if ((triCorner == 1.0)) {
        let c1Raw = (triF + 1.0);
        let c1SegF = floor((c1Raw / 44.0));
        let c1Idx = (c1Raw - (c1SegF * 44.0));
        let c1CornF = floor((c1Idx / 11.0));
        let c1Corn = min(c1CornF, 3.0);
        let c1Local = (c1Idx - (c1Corn * 11.0));
        let c1SignX = select(1.0, (-1.0), ((c1Corn == 1.0) || (c1Corn == 2.0)));
        let c1SignY = select(1.0, (-1.0), (c1Corn >= 2.0));
        let c1Theta = ((c1Corn * 1.57079632679) + ((c1Local / 10.0) * 1.57079632679));
        px = ((c1SignX * insetX) + (cos(c1Theta) * cr));
        py = ((c1SignY * insetY) + (sin(c1Theta) * cr));
      } else {
        if ((triCorner == 2.0)) {
          let c2SegF = floor((triF / 44.0));
          let c2Idx = (triF - (c2SegF * 44.0));
          let c2CornF = floor((c2Idx / 11.0));
          let c2Corn = min(c2CornF, 3.0);
          let c2Local = (c2Idx - (c2Corn * 11.0));
          let c2SignX = select(1.0, (-1.0), ((c2Corn == 1.0) || (c2Corn == 2.0)));
          let c2SignY = select(1.0, (-1.0), (c2Corn >= 2.0));
          let c2Theta = ((c2Corn * 1.57079632679) + ((c2Local / 10.0) * 1.57079632679));
          px = ((c2SignX * insetX) + (cos(c2Theta) * cr));
          py = ((c2SignY * insetY) + (sin(c2Theta) * cr));
        }
      }
      wx = px;
      wy = flY;
      wz = py;
      tileX = (px * 0.42);
      tileY = (py * 0.42);
      faceOut = 0.0;
    } else {
      let localIndex = (fi - 132.0);
      let segRaw = floor((localIndex / 6.0));
      let segSegF = floor((segRaw / 44.0));
      let segment = (segRaw - (segSegF * 44.0));
      let wallCorner = (localIndex - (segRaw * 6.0));
      let quadU = select(0.0, 1.0, (((wallCorner == 1.0) || (wallCorner == 2.0)) || (wallCorner == 4.0)));
      let quadV = select(0.0, 1.0, (((wallCorner == 2.0) || (wallCorner == 4.0)) || (wallCorner == 5.0)));
      let aCornF = min(floor((segment / 11.0)), 3.0);
      let aLocal = (segment - (aCornF * 11.0));
      let aSignX = select(1.0, (-1.0), ((aCornF == 1.0) || (aCornF == 2.0)));
      let aSignY = select(1.0, (-1.0), (aCornF >= 2.0));
      let aTheta = ((aCornF * 1.57079632679) + ((aLocal / 10.0) * 1.57079632679));
      let aPX = ((aSignX * insetX) + (cos(aTheta) * cr));
      let aPY = ((aSignY * insetY) + (sin(aTheta) * cr));
      let bRaw = (segment + 1.0);
      let bSegF = floor((bRaw / 44.0));
      let bIdx = (bRaw - (bSegF * 44.0));
      let bCornF = min(floor((bIdx / 11.0)), 3.0);
      let bLocal = (bIdx - (bCornF * 11.0));
      let bSignX = select(1.0, (-1.0), ((bCornF == 1.0) || (bCornF == 2.0)));
      let bSignY = select(1.0, (-1.0), (bCornF >= 2.0));
      let bTheta = ((bCornF * 1.57079632679) + ((bLocal / 10.0) * 1.57079632679));
      let bPX = ((bSignX * insetX) + (cos(bTheta) * cr));
      let bPY = ((bSignY * insetY) + (sin(bTheta) * cr));
      let point = vec2<f32>(mix(aPX, bPX, quadU), mix(aPY, bPY, quadU));
      let inset = vec2<f32>(insetX, insetY);
      let absPoint = abs(point);
      var outward : vec2<f32> = vec2<f32>(0.0, 1.0);
      if ((((absPoint.x > insetX) && (absPoint.y > insetY)) && (cr > 0.0001))) {
        outward = normalize((point - (sign(point) * inset)));
      } else {
        if (((absPoint.x / max(hw, 0.001)) > (absPoint.y / max(hl, 0.001)))) {
          outward = vec2<f32>(sign(point.x), 0.0);
        } else {
          outward = vec2<f32>(0.0, sign(point.y));
        }
      }
      wx = point.x;
      wy = mix(flY, rimY, quadV);
      wz = point.y;
      nx = (-outward.x);
      ny = 0.0;
      nz = (-outward.y);
      tileX = ((segment + quadU) * 0.18);
      tileY = (wy * 0.72);
      faceOut = 5.0;
    }
  } else {
    let fv = floor((fi / 6.0));
    let faceF = min(fv, 4.0);
    let cu = (fi - (fv * 6.0));
    let uf = select(select(select(0.0, 1.0, (cu == 4.0)), 1.0, (cu == 2.0)), 1.0, (cu == 1.0));
    let vf = select(select(select(0.0, 1.0, (cu == 5.0)), 1.0, (cu == 4.0)), 1.0, (cu == 2.0));
    wx = select(select(select(mix((-hw), hw, uf), (-hw), (faceF == 4.0)), hw, (faceF == 3.0)), mix(hw, (-hw), uf), (faceF == 2.0));
    wy = select(mix(flY, rimY, vf), flY, (faceF == 0.0));
    wz = select(select(select(select(mix((-hl), hl, vf), mix((-hl), hl, uf), (faceF == 4.0)), mix(hl, (-hl), uf), (faceF == 3.0)), (-hl), (faceF == 2.0)), hl, (faceF == 1.0));
    nx = select(select(0.0, 1.0, (faceF == 4.0)), (-1.0), (faceF == 3.0));
    ny = select(0.0, 1.0, (faceF == 0.0));
    nz = select(select(0.0, 1.0, (faceF == 2.0)), (-1.0), (faceF == 1.0));
    tileX = select(select((wx * 0.42), (wz * 0.42), (faceF == 4.0)), (wz * 0.42), (faceF == 3.0));
    tileY = select((wy * 0.72), (wz * 0.42), (faceF == 0.0));
    faceOut = faceF;
  }
  let duw = max((u.poolWidth * 2.0), 0.001);
  let dul = max((u.poolLength * 2.0), 0.001);
  out.vWorldPos = vec3<f32>(wx, wy, wz);
  out.vNormal = vec3<f32>(nx, ny, nz);
  out.vTileUV = vec2<f32>(tileX, tileY);
  out.vWaterUV = vec2<f32>(((wx / duw) + 0.5), ((wz / dul) + 0.5));
  out.vFace = faceOut;
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
