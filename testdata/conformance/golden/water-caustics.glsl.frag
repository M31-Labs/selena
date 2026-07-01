#extension GL_OES_standard_derivatives : enable
precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolWidth;
uniform float poolLength;
uniform float poolHeight;
uniform float normalScale;
uniform float resolution;
uniform float time;
uniform float objectKind;
uniform float objectCount;
uniform float opticsEnable;
uniform vec3 lightDir;
uniform vec3 objectCenter;
uniform vec4 objectHalfRadius;
uniform vec4 spheres[32];
uniform highp sampler2D stateTex;
varying vec2 vUv;

void main() {
  vec2 uv = clamp(vUv, vec2(0.0, 0.0), vec2(1.0, 1.0));
  float texel = (1.0 / max(resolution, 1.0));
  vec4 c = texture2D(stateTex, uv);
  vec4 e = texture2D(stateTex, (uv + vec2(texel, 0.0)));
  vec4 wv = texture2D(stateTex, (uv - vec2(texel, 0.0)));
  vec4 nn = texture2D(stateTex, (uv + vec2(0.0, texel)));
  vec4 ss = texture2D(stateTex, (uv - vec2(0.0, texel)));
  vec3 ldir = normalize(lightDir);
  vec3 waterNormal = normalize(vec3((c.z * normalScale), 1.0, (c.w * normalScale)));
  vec3 causticNormal = normalize(vec3(((c.z * normalScale) * 0.5), 1.0, ((c.w * normalScale) * 0.5)));
  vec3 refracted = refract((-ldir), waterNormal, (1.0 / 1.333));
  vec3 causticRay = refract((-ldir), causticNormal, (1.0 / 1.333));
  vec3 flatRay = refract((-ldir), vec3(0.0, 1.0, 0.0), (1.0 / 1.333));
  vec3 origin = vec3((((uv.x - 0.5) * poolWidth) * 2.0), 0.0, (((uv.y - 0.5) * poolLength) * 2.0));
  vec3 originH = vec3(origin.x, (c.x * poolHeight), origin.z);
  vec3 oldPos = (origin + (flatRay * (((-poolHeight) - origin.y) / ((abs(flatRay.y) > 0.0001) ? flatRay.y : ((flatRay.y >= 0.0) ? 0.0001 : (-0.0001))))));
  vec3 newPos = (originH + (causticRay * (((-poolHeight) - originH.y) / ((abs(causticRay.y) > 0.0001) ? causticRay.y : ((causticRay.y >= 0.0) ? 0.0001 : (-0.0001))))));
  float oldArea = max((length(dFdx(oldPos)) * length(dFdy(oldPos))), 0.000001);
  float newArea = max((length(dFdx(newPos)) * length(dFdy(newPos))), 0.000001);
  float convergence = abs(((((e.x + wv.x) + nn.x) + ss.x) - (c.x * 4.0)));
  vec3 slopeRay = normalize(vec3((-refracted.x), max(refracted.y, 0.05), (-refracted.z)));
  float slopeFocus = max(0.0, dot(slopeRay, waterNormal));
  float shimmer = (0.5 + (0.5 * sin(((((uv.x * 41.0) + (uv.y * 37.0)) + (time * 2.4)) + (c.x * 180.0)))));
  float areaFocus = clamp(((oldArea / newArea) * 0.2), 0.0, 4.0);
  float slopeMag = length(c.zw);
  float intensity = (areaFocus * (0.68 + (0.32 * smoothstep(0.001, 0.028, ((convergence * 0.72) + (slopeMag * 0.035))))));
  intensity = ((intensity * (0.52 + (0.48 * shimmer))) * (0.58 + (0.42 * slopeFocus)));
  vec2 centerUV = vec2(((objectCenter.x * 0.5) + 0.5), ((objectCenter.z * 0.5) + 0.5));
  vec2 aspect = vec2(max((poolWidth / max(poolLength, 0.001)), 0.001), 1.0);
  float compMask = 0.0;
  for (int i = 0; (i < 32); i = (i + 1)) {
    if (((objectKind >= 2.5) && (float(i) < objectCount))) {
      vec4 sp = spheres[i];
      vec2 suv = (centerUV + vec2((sp.x * 0.5), (sp.z * 0.5)));
      float rad = max((sp.w * 0.72), 0.012);
      float dd = length(((uv - suv) * aspect));
      compMask = max(compMask, (1.0 - smoothstep(rad, (rad + max((rad * 1.25), 0.018)), dd)));
    }
  }
  float singleR0 = max((objectHalfRadius.w * 0.55), 0.018);
  float singleRC = max((max(objectHalfRadius.x, objectHalfRadius.z) * 0.6), singleR0);
  float singleR = ((objectKind > 1.5) ? singleRC : singleR0);
  float singleD = length(((uv - centerUV) * aspect));
  float singleMask = (1.0 - smoothstep(singleR, (singleR + max((singleR * 1.2), 0.02)), singleD));
  float maskRaw = ((objectKind >= 2.5) ? compMask : singleMask);
  bool maskOn = ((objectKind >= 0.5) && (opticsEnable > 0.0));
  float shadowMask = (maskOn ? maskRaw : 0.0);
  float sphereShadow = ((((objectKind >= 0.5) && (objectKind < 1.5)) && (opticsEnable > 0.0)) ? (1.0 - mix(1.0, clamp((1.0 / (1.0 + exp((-(1.0 + ((dot(cross(((objectCenter - newPos) / max(objectHalfRadius.w, 0.0001)), flatRay), cross(((objectCenter - newPos) / max(objectHalfRadius.w, 0.0001)), flatRay)) - 1.0) / max((0.05 + (dot(((objectCenter - newPos) / max(objectHalfRadius.w, 0.0001)), (-flatRay)) * 0.025)), 0.0001))))))), 0.0, 1.0), clamp((dot(((objectCenter - newPos) / max(objectHalfRadius.w, 0.0001)), (-flatRay)) * 2.0), 0.0, 1.0))) : 0.0);
  vec3 shadowRay = (-flatRay);
  vec3 sright = normalize(cross(shadowRay, vec3(0.0, 1.0, 0.0)));
  vec3 sup = normalize(cross(sright, shadowRay));
  bool cubeActive = (((objectKind >= 1.5) && (objectKind < 2.5)) && (opticsEnable > 0.0));
  vec3 cubeHalf = objectHalfRadius.xyz;
  float cubeOcc = 0.0;
  for (int cx = 0; (cx < 3); cx = (cx + 1)) {
    for (int cy = 0; (cy < 3); cy = (cy + 1)) {
      float fx = (float(cx) - 1.0);
      float fy = (float(cy) - 1.0);
      vec3 so = ((newPos + ((sright * fx) * 0.025)) + ((sup * fy) * 0.025));
      cubeOcc = (cubeOcc + (step(0.0, vec2(max(max(min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z), min(min(max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z)).y) * step(vec2(max(max(min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z), min(min(max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z)).x, vec2(max(max(min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), min((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z), min(min(max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).x, max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).y), max((((objectCenter - max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001)))), (((objectCenter + max(cubeHalf, vec3(0.0001, 0.0001, 0.0001))) - so) * vec3((1.0 / ((abs(shadowRay.x) > 0.000001) ? shadowRay.x : 0.000001)), (1.0 / ((abs(shadowRay.y) > 0.000001) ? shadowRay.y : 0.000001)), (1.0 / ((abs(shadowRay.z) > 0.000001) ? shadowRay.z : 0.000001))))).z)).y)));
    }
  }
  float cubeShadow = (cubeActive ? (cubeOcc / 9.0) : 0.0);
  float shadow = max(max(shadowMask, sphereShadow), cubeShadow);
  float lit = (intensity * (1.0 - (shadow * 0.82)));
  vec3 warm = vec3(1.0, 0.78, 0.42);
  vec3 cool = vec3(0.44, 0.95, 1.0);
  gl_FragColor = vec4((mix(cool, warm, clamp((lit * 1.8), 0.0, 1.0)) * lit), 1.0);
}
