struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
  poolWidth : f32,
  poolLength : f32,
  poolHeight : f32,
  normalScale : f32,
  opticsEnable : f32,
  resolution : f32,
  time : f32,
  objectKind : f32,
  objectCount : f32,
  lightDir : vec3<f32>,
  objectCenter : vec3<f32>,
  objectHalfRadius : vec4<f32>,
  spheres : array<vec4<f32>, 32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct StateGrid {
  gridWidth  : u32,
  gridHeight : u32,
};
@group(0) @binding(1) var<uniform> _stateGrid : StateGrid;
@group(0) @binding(2) var<storage, read> _inState : array<vec4<f32>>;

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) vUv : vec2<f32>,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex : u32) -> VertexOutput {
  var out : VertexOutput;
  let fi = f32(vertexIndex);
  let ox = select(0.0, 1.0, (((fi == 1.0) || (fi == 2.0)) || (fi == 4.0)));
  let oy = select(0.0, 1.0, (((fi == 2.0) || (fi == 4.0)) || (fi == 5.0)));
  out.vUv = vec2<f32>(ox, oy);
  out.position = vec4<f32>(((ox * 2.0) - 1.0), ((oy * 2.0) - 1.0), 0.0, 1.0);
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  let uv = clamp(in.vUv, vec2<f32>(0.0, 0.0), vec2<f32>(1.0, 1.0));
  let texel = (1.0 / max(u.resolution, 1.0));
  let c = _inState[min(u32((uv).x * f32(_stateGrid.gridWidth)) + u32((uv).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let e = _inState[min(u32(((uv + vec2<f32>(texel, 0.0))).x * f32(_stateGrid.gridWidth)) + u32(((uv + vec2<f32>(texel, 0.0))).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let wv = _inState[min(u32(((uv - vec2<f32>(texel, 0.0))).x * f32(_stateGrid.gridWidth)) + u32(((uv - vec2<f32>(texel, 0.0))).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let nn = _inState[min(u32(((uv + vec2<f32>(0.0, texel))).x * f32(_stateGrid.gridWidth)) + u32(((uv + vec2<f32>(0.0, texel))).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let ss = _inState[min(u32(((uv - vec2<f32>(0.0, texel))).x * f32(_stateGrid.gridWidth)) + u32(((uv - vec2<f32>(0.0, texel))).y * f32(_stateGrid.gridHeight)) * _stateGrid.gridWidth, _stateGrid.gridWidth * _stateGrid.gridHeight - 1u)];
  let ldir = normalize(u.lightDir);
  let waterNormal = normalize(vec3<f32>((c.z * u.normalScale), 1.0, (c.w * u.normalScale)));
  let causticNormal = normalize(vec3<f32>(((c.z * u.normalScale) * 0.5), 1.0, ((c.w * u.normalScale) * 0.5)));
  let refracted = refract((-ldir), waterNormal, (1.0 / 1.333));
  let causticRay = refract((-ldir), causticNormal, (1.0 / 1.333));
  let flatRay = refract((-ldir), vec3<f32>(0.0, 1.0, 0.0), (1.0 / 1.333));
  let origin = vec3<f32>((((uv.x - 0.5) * u.poolWidth) * 2.0), 0.0, (((uv.y - 0.5) * u.poolLength) * 2.0));
  let originH = vec3<f32>(origin.x, (c.x * u.poolHeight), origin.z);
  let oldPos = (origin + (flatRay * (((-u.poolHeight) - origin.y) / select(select((-0.0001), 0.0001, (flatRay.y >= 0.0)), flatRay.y, (abs(flatRay.y) > 0.0001)))));
  let newPos = (originH + (causticRay * (((-u.poolHeight) - originH.y) / select(select((-0.0001), 0.0001, (causticRay.y >= 0.0)), causticRay.y, (abs(causticRay.y) > 0.0001)))));
  let oldArea = max((length(dpdx(oldPos)) * length(dpdy(oldPos))), 0.000001);
  let newArea = max((length(dpdx(newPos)) * length(dpdy(newPos))), 0.000001);
  let convergence = abs(((((e.x + wv.x) + nn.x) + ss.x) - (c.x * 4.0)));
  let slopeRay = normalize(vec3<f32>((-refracted.x), max(refracted.y, 0.05), (-refracted.z)));
  let slopeFocus = max(0.0, dot(slopeRay, waterNormal));
  let shimmer = (0.5 + (0.5 * sin(((((uv.x * 41.0) + (uv.y * 37.0)) + (u.time * 2.4)) + (c.x * 180.0)))));
  let areaFocus = clamp(((oldArea / newArea) * 0.2), 0.0, 4.0);
  let slopeMag = length(c.zw);
  var intensity : f32 = (areaFocus * (0.68 + (0.32 * smoothstep(0.001, 0.028, ((convergence * 0.72) + (slopeMag * 0.035))))));
  intensity = ((intensity * (0.52 + (0.48 * shimmer))) * (0.58 + (0.42 * slopeFocus)));
  let centerUV = vec2<f32>(((u.objectCenter.x * 0.5) + 0.5), ((u.objectCenter.z * 0.5) + 0.5));
  let aspect = vec2<f32>(max((u.poolWidth / max(u.poolLength, 0.001)), 0.001), 1.0);
  var compMask : f32 = 0.0;
  for (var i : i32 = 0i; (i < 32i); i = (i + 1i)) {
    if (((u.objectKind >= 2.5) && (f32(i) < u.objectCount))) {
      let sp = u.spheres[i];
      let suv = (centerUV + vec2<f32>((sp.x * 0.5), (sp.z * 0.5)));
      let rad = max((sp.w * 0.72), 0.012);
      let dd = length(((uv - suv) * aspect));
      compMask = max(compMask, (1.0 - smoothstep(rad, (rad + max((rad * 1.25), 0.018)), dd)));
    }
  }
  let singleR0 = max((u.objectHalfRadius.w * 0.55), 0.018);
  let singleRC = max((max(u.objectHalfRadius.x, u.objectHalfRadius.z) * 0.6), singleR0);
  let singleR = select(singleR0, singleRC, (u.objectKind > 1.5));
  let singleD = length(((uv - centerUV) * aspect));
  let singleMask = (1.0 - smoothstep(singleR, (singleR + max((singleR * 1.2), 0.02)), singleD));
  let maskRaw = select(singleMask, compMask, (u.objectKind >= 2.5));
  let maskOn = ((u.objectKind >= 0.5) && (u.opticsEnable > 0.0));
  let shadowMask = select(0.0, maskRaw, maskOn);
  let sphereShadow = select(0.0, (1.0 - mix(1.0, clamp((1.0 / (1.0 + exp((-(1.0 + ((dot(cross(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), flatRay), cross(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), flatRay)) - 1.0) / max((0.05 + (dot(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), (-flatRay)) * 0.025)), 0.0001))))))), 0.0, 1.0), clamp((dot(((u.objectCenter - newPos) / max(u.objectHalfRadius.w, 0.0001)), (-flatRay)) * 2.0), 0.0, 1.0))), (((u.objectKind >= 0.5) && (u.objectKind < 1.5)) && (u.opticsEnable > 0.0)));
  let shadowRay = (-flatRay);
  let sright = normalize(cross(shadowRay, vec3<f32>(0.0, 1.0, 0.0)));
  let sup = normalize(cross(sright, shadowRay));
  let cubeActive = (((u.objectKind >= 1.5) && (u.objectKind < 2.5)) && (u.opticsEnable > 0.0));
  let cubeHalf = u.objectHalfRadius.xyz;
  var cubeOcc : f32 = 0.0;
  for (var cx : i32 = 0i; (cx < 3i); cx = (cx + 1i)) {
    for (var cy : i32 = 0i; (cy < 3i); cy = (cy + 1i)) {
      let fx = (f32(cx) - 1.0);
      let fy = (f32(cy) - 1.0);
      let so = ((newPos + ((sright * fx) * 0.025)) + ((sup * fy) * 0.025));
      cubeOcc = (cubeOcc + (step(0.0, vec2<f32>(max(max(min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).x, min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).y), min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).z), min(min(max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).x, max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).y), max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).z)).y) * step(vec2<f32>(max(max(min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).x, min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).y), min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).z), min(min(max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).x, max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).y), max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).z)).x, vec2<f32>(max(max(min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).x, min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).y), min((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).z), min(min(max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).x, max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).y), max((((u.objectCenter - max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001))))), (((u.objectCenter + max(cubeHalf, vec3<f32>(0.0001, 0.0001, 0.0001))) - so) * vec3<f32>((1.0 / select(0.000001, shadowRay.x, (abs(shadowRay.x) > 0.000001))), (1.0 / select(0.000001, shadowRay.y, (abs(shadowRay.y) > 0.000001))), (1.0 / select(0.000001, shadowRay.z, (abs(shadowRay.z) > 0.000001)))))).z)).y)));
    }
  }
  let cubeShadow = select(0.0, (cubeOcc / 9.0), cubeActive);
  let shadow = max(max(shadowMask, sphereShadow), cubeShadow);
  let lit = (intensity * (1.0 - (shadow * 0.82)));
  let warm = vec3<f32>(1.0, 0.78, 0.42);
  let cool = vec3<f32>(0.44, 0.95, 1.0);
  return vec4<f32>((mix(cool, warm, clamp((lit * 1.8), 0.0, 1.0)) * lit), 1.0);
}
