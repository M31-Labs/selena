#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolHeight;
uniform vec4 baseColor;
uniform float isTexturePass;
uniform float texturePassMode;
uniform vec3 lightDir;
uniform highp sampler2D stateTex;
in vec3 worldPos;
in vec2 vUv;
out vec4 fragColor;

void main() {
  vec4 info = texture(stateTex, vUv);
  float waterHeight = (info.x * poolHeight);
  float caustic = info.y;
  bool belowWater = (worldPos.y < waterHeight);
  if ((((isTexturePass > 0.5) && (texturePassMode == 2.0)) && belowWater)) {
    discard;
  }
  vec3 lightN = normalize(lightDir);
  vec3 refr = refract((-lightN), vec3(0.0, 1.0, 0.0), (1.0 / 1.333));
  vec3 up = vec3(0.0, 1.0, 0.0);
  bool submerged = (worldPos.y < waterHeight);
  float diffuse = (max(dot((-refr), up), 0.0) * 0.5);
  if (submerged) {
    diffuse = (((diffuse + 0.06) * caustic) * 4.0);
  }
  vec3 underwater = vec3(0.4, 0.9, 1.0);
  vec3 col = (baseColor.rgb + vec3(diffuse, diffuse, diffuse));
  if (submerged) {
    col = ((col * underwater) * 1.2);
  }
  fragColor = vec4(col, baseColor.a);
}
