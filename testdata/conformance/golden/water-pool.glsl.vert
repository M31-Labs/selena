attribute float a_vertexIndex;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolWidth;
uniform float poolLength;
uniform float poolHeight;
uniform float cornerRadius;
uniform float poolShape;
uniform vec3 lightDir;
uniform highp sampler2D stateTex;
varying vec3 vWorldPos;
varying vec3 vNormal;
varying vec2 vTileUV;
varying vec2 vWaterUV;
varying float vFace;

void main() {
  float vertexIndex = a_vertexIndex;
  float fi = float(vertexIndex);
  float hw = max(poolWidth, 0.001);
  float hl = max(poolLength, 0.001);
  float flY = (-max(poolHeight, 0.001));
  float rimY = max((poolHeight * 0.1667), 0.025);
  float maxCornerRadius = max(0.0, (min(hw, hl) - 0.001));
  float cr = clamp(cornerRadius, 0.0, maxCornerRadius);
  bool roundedActive = ((poolShape > 0.5) && (cr > 0.0001));
  float wx = 0.0;
  float wy = 0.0;
  float wz = 0.0;
  float nx = 0.0;
  float ny = 1.0;
  float nz = 0.0;
  float tileX = 0.0;
  float tileY = 0.0;
  float faceOut = 0.0;
  if (roundedActive) {
    float insetX = max((hw - cr), 0.001);
    float insetY = max((hl - cr), 0.001);
    if ((fi < 132.0)) {
      float triF = floor((fi / 3.0));
      float triCorner = (fi - (triF * 3.0));
      float px = 0.0;
      float py = 0.0;
      if ((triCorner == 1.0)) {
        float c1Raw = (triF + 1.0);
        float c1SegF = floor((c1Raw / 44.0));
        float c1Idx = (c1Raw - (c1SegF * 44.0));
        float c1CornF = floor((c1Idx / 11.0));
        float c1Corn = min(c1CornF, 3.0);
        float c1Local = (c1Idx - (c1Corn * 11.0));
        float c1SignX = (((c1Corn == 1.0) || (c1Corn == 2.0)) ? (-1.0) : 1.0);
        float c1SignY = ((c1Corn >= 2.0) ? (-1.0) : 1.0);
        float c1Theta = ((c1Corn * 1.57079632679) + ((c1Local / 10.0) * 1.57079632679));
        px = ((c1SignX * insetX) + (cos(c1Theta) * cr));
        py = ((c1SignY * insetY) + (sin(c1Theta) * cr));
      } else {
        if ((triCorner == 2.0)) {
          float c2SegF = floor((triF / 44.0));
          float c2Idx = (triF - (c2SegF * 44.0));
          float c2CornF = floor((c2Idx / 11.0));
          float c2Corn = min(c2CornF, 3.0);
          float c2Local = (c2Idx - (c2Corn * 11.0));
          float c2SignX = (((c2Corn == 1.0) || (c2Corn == 2.0)) ? (-1.0) : 1.0);
          float c2SignY = ((c2Corn >= 2.0) ? (-1.0) : 1.0);
          float c2Theta = ((c2Corn * 1.57079632679) + ((c2Local / 10.0) * 1.57079632679));
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
      float localIndex = (fi - 132.0);
      float segRaw = floor((localIndex / 6.0));
      float segSegF = floor((segRaw / 44.0));
      float segment = (segRaw - (segSegF * 44.0));
      float wallCorner = (localIndex - (segRaw * 6.0));
      float quadU = ((((wallCorner == 1.0) || (wallCorner == 2.0)) || (wallCorner == 4.0)) ? 1.0 : 0.0);
      float quadV = ((((wallCorner == 2.0) || (wallCorner == 4.0)) || (wallCorner == 5.0)) ? 1.0 : 0.0);
      float aCornF = min(floor((segment / 11.0)), 3.0);
      float aLocal = (segment - (aCornF * 11.0));
      float aSignX = (((aCornF == 1.0) || (aCornF == 2.0)) ? (-1.0) : 1.0);
      float aSignY = ((aCornF >= 2.0) ? (-1.0) : 1.0);
      float aTheta = ((aCornF * 1.57079632679) + ((aLocal / 10.0) * 1.57079632679));
      float aPX = ((aSignX * insetX) + (cos(aTheta) * cr));
      float aPY = ((aSignY * insetY) + (sin(aTheta) * cr));
      float bRaw = (segment + 1.0);
      float bSegF = floor((bRaw / 44.0));
      float bIdx = (bRaw - (bSegF * 44.0));
      float bCornF = min(floor((bIdx / 11.0)), 3.0);
      float bLocal = (bIdx - (bCornF * 11.0));
      float bSignX = (((bCornF == 1.0) || (bCornF == 2.0)) ? (-1.0) : 1.0);
      float bSignY = ((bCornF >= 2.0) ? (-1.0) : 1.0);
      float bTheta = ((bCornF * 1.57079632679) + ((bLocal / 10.0) * 1.57079632679));
      float bPX = ((bSignX * insetX) + (cos(bTheta) * cr));
      float bPY = ((bSignY * insetY) + (sin(bTheta) * cr));
      vec2 point = vec2(mix(aPX, bPX, quadU), mix(aPY, bPY, quadU));
      vec2 inset = vec2(insetX, insetY);
      vec2 absPoint = abs(point);
      vec2 outward = vec2(0.0, 1.0);
      if ((((absPoint.x > insetX) && (absPoint.y > insetY)) && (cr > 0.0001))) {
        outward = normalize((point - (sign(point) * inset)));
      } else {
        if (((absPoint.x / max(hw, 0.001)) > (absPoint.y / max(hl, 0.001)))) {
          outward = vec2(sign(point.x), 0.0);
        } else {
          outward = vec2(0.0, sign(point.y));
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
    float fv = floor((fi / 6.0));
    float faceF = min(fv, 4.0);
    float cu = (fi - (fv * 6.0));
    float uf = ((cu == 1.0) ? 1.0 : ((cu == 2.0) ? 1.0 : ((cu == 4.0) ? 1.0 : 0.0)));
    float vf = ((cu == 2.0) ? 1.0 : ((cu == 4.0) ? 1.0 : ((cu == 5.0) ? 1.0 : 0.0)));
    wx = ((faceF == 2.0) ? mix(hw, (-hw), uf) : ((faceF == 3.0) ? hw : ((faceF == 4.0) ? (-hw) : mix((-hw), hw, uf))));
    wy = ((faceF == 0.0) ? flY : mix(flY, rimY, vf));
    wz = ((faceF == 1.0) ? hl : ((faceF == 2.0) ? (-hl) : ((faceF == 3.0) ? mix(hl, (-hl), uf) : ((faceF == 4.0) ? mix((-hl), hl, uf) : mix((-hl), hl, vf)))));
    nx = ((faceF == 3.0) ? (-1.0) : ((faceF == 4.0) ? 1.0 : 0.0));
    ny = ((faceF == 0.0) ? 1.0 : 0.0);
    nz = ((faceF == 1.0) ? (-1.0) : ((faceF == 2.0) ? 1.0 : 0.0));
    tileX = ((faceF == 3.0) ? (wz * 0.42) : ((faceF == 4.0) ? (wz * 0.42) : (wx * 0.42)));
    tileY = ((faceF == 0.0) ? (wz * 0.42) : (wy * 0.72));
    faceOut = faceF;
  }
  float duw = max((poolWidth * 2.0), 0.001);
  float dul = max((poolLength * 2.0), 0.001);
  vWorldPos = vec3(wx, wy, wz);
  vNormal = vec3(nx, ny, nz);
  vTileUV = vec2(tileX, tileY);
  vWaterUV = vec2(((wx / duw) + 0.5), ((wz / dul) + 0.5));
  vFace = faceOut;
  gl_Position = (mvp * vec4(wx, wy, wz, 1.0));
}
