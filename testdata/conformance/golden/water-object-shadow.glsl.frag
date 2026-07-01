precision mediump float;
varying vec2 v_uv;

uniform sampler2D _sceneColor;
uniform sampler2D _sceneDepth;
uniform float objectKind;
uniform float objectEnabled;
uniform vec3 lightDir;
uniform float poolWidth;
uniform float poolLength;
uniform float objectCenterX;
uniform float objectCenterZ;
uniform float objectRadius;
uniform float objectHalfX;
uniform float objectHalfZ;

void main() {
  vec3 lnorm = normalize(lightDir);
  float uvX = clamp((v_uv.x - (lnorm.x * 0.025)), 0.0, 1.0);
  float uvY = clamp((v_uv.y + (lnorm.z * 0.025)), 0.0, 1.0);
  vec2 centerUV = vec2(((objectCenterX * 0.5) + 0.5), ((objectCenterZ * 0.5) + 0.5));
  vec2 aspect = vec2(max((poolWidth / max(poolLength, 0.001)), 0.001), 1.0);
  float dd = length((vec2((uvX - centerUV.x), (uvY - centerUV.y)) * aspect));
  float sphR = max((objectRadius * 0.55), 0.018);
  float cubeR = max((max(objectHalfX, objectHalfZ) * 0.6), sphR);
  float radius = ((objectKind > 1.5) ? cubeR : sphR);
  float mask = 0.0;
  float core = 0.0;
  if (((objectKind >= 0.5) && (objectEnabled > 0.0))) {
    mask = (1.0 - smoothstep(radius, (radius + max((radius * 1.2), 0.02)), dd));
    core = (1.0 - smoothstep((radius * 0.38), radius, dd));
  }
  float shadow = (mask * (0.42 + (core * 0.58)));
  gl_FragColor = vec4(vec3(shadow, shadow, shadow), 1.0);
}
