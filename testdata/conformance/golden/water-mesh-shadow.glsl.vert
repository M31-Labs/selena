attribute vec3 position;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolHalfW;
uniform float poolHalfL;
uniform vec3 lightDir;

void main() {
  vec3 pos = position;
  vec3 lnorm = normalize(lightDir);
  vec3 refr = refract((-lnorm), vec3(0.0, 1.0, 0.0), (1.0 / 1.333));
  float fallY = ((refr.y >= 0.0) ? 0.0001 : (-0.0001));
  float refY = ((abs(refr.y) > 0.0001) ? refr.y : fallY);
  float pX = (0.75 * (pos.x - ((pos.y * refr.x) / refY)));
  float pZ = (0.75 * (pos.z - ((pos.y * refr.z) / refY)));
  float clipX = (pX / max(poolHalfW, 0.0001));
  float clipZ = (pZ / max(poolHalfL, 0.0001));
  gl_Position = vec4(clipX, clipZ, 0.0, 1.0);
}
