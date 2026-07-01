#version 300 es
precision highp float;
in vec2 v_uv;

uniform sampler2D _sceneColor;
uniform sampler2D _sceneDepth;
uniform float sphereCount;
uniform float objectEnabled;
uniform float objectTop;
uniform vec3 lightDir;
uniform float poolWidth;
uniform float poolLength;
uniform float objectCenterX;
uniform float objectCenterZ;
uniform vec4 spheres[32];
out vec4 fragColor;

void main() {
  vec3 lnorm = normalize(lightDir);
  float uvX = clamp((v_uv.x - (lnorm.x * 0.025)), 0.0, 1.0);
  float uvY = clamp((v_uv.y + (lnorm.z * 0.025)), 0.0, 1.0);
  vec2 aspect = vec2(max((poolWidth / max(poolLength, 0.001)), 0.001), 1.0);
  float mask = 0.0;
  float core = 0.0;
  for (int i = 0; (i < 32); i = (i + 1)) {
    if (((objectEnabled > 0.0) && (float(i) < sphereCount))) {
      vec4 sphere = spheres[i];
      float sphereUVx = (((objectCenterX + sphere.x) * 0.5) + 0.5);
      float sphereUVy = (((objectCenterZ + sphere.z) * 0.5) + 0.5);
      float radius = max((sphere.w * 0.58), 0.012);
      vec2 dvec = vec2(((uvX - sphereUVx) * aspect.x), ((uvY - sphereUVy) * aspect.y));
      float dd = length(dvec);
      float localMask = (1.0 - smoothstep(radius, (radius + max((radius * 1.35), 0.018)), dd));
      mask = max(mask, localMask);
      core = max(core, (1.0 - smoothstep((radius * 0.42), radius, dd)));
    }
  }
  float clipped = smoothstep((-0.08), 0.16, objectTop);
  float shadow = ((mask * (0.42 + (0.58 * core))) * clipped);
  fragColor = vec4(vec3(shadow, shadow, shadow), 1.0);
}
