#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform vec3 edgeColor;
uniform float progress;
uniform float noiseScale;
uniform float edgeWidth;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
in vec3 vWorldNormal;
in vec3 vWorldPos;
out vec4 fragColor;

void main() {
  vec3 selenaLib0 = (vWorldPos * noiseScale);
  vec2 selenaLib1 = selenaLib0.xy;
  vec2 selenaLib2 = floor(selenaLib1);
  vec2 selenaLib3 = fract(selenaLib1);
  vec2 selenaLib4 = ((selenaLib3 * selenaLib3) * (3.0 - (2.0 * selenaLib3)));
  vec2 selenaLib5 = selenaLib2;
  float selenaLib6 = fract((sin(dot(selenaLib5, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib7 = selenaLib6;
  vec2 selenaLib8 = (selenaLib2 + vec2(1.0, 0.0));
  float selenaLib9 = fract((sin(dot(selenaLib8, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib10 = selenaLib9;
  vec2 selenaLib11 = (selenaLib2 + vec2(0.0, 1.0));
  float selenaLib12 = fract((sin(dot(selenaLib11, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib13 = selenaLib12;
  vec2 selenaLib14 = (selenaLib2 + vec2(1.0, 1.0));
  float selenaLib15 = fract((sin(dot(selenaLib14, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib16 = selenaLib15;
  float selenaLib17 = mix(mix(selenaLib7, selenaLib10, selenaLib4.x), mix(selenaLib13, selenaLib16, selenaLib4.x), selenaLib4.y);
  vec2 selenaLib18 = (selenaLib0.zx + vec2(17.0, 31.0));
  vec2 selenaLib19 = floor(selenaLib18);
  vec2 selenaLib20 = fract(selenaLib18);
  vec2 selenaLib21 = ((selenaLib20 * selenaLib20) * (3.0 - (2.0 * selenaLib20)));
  vec2 selenaLib22 = selenaLib19;
  float selenaLib23 = fract((sin(dot(selenaLib22, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib24 = selenaLib23;
  vec2 selenaLib25 = (selenaLib19 + vec2(1.0, 0.0));
  float selenaLib26 = fract((sin(dot(selenaLib25, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib27 = selenaLib26;
  vec2 selenaLib28 = (selenaLib19 + vec2(0.0, 1.0));
  float selenaLib29 = fract((sin(dot(selenaLib28, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib30 = selenaLib29;
  vec2 selenaLib31 = (selenaLib19 + vec2(1.0, 1.0));
  float selenaLib32 = fract((sin(dot(selenaLib31, vec2(127.1, 311.7))) * 43758.5453));
  float selenaLib33 = selenaLib32;
  float selenaLib34 = mix(mix(selenaLib24, selenaLib27, selenaLib21.x), mix(selenaLib30, selenaLib33, selenaLib21.x), selenaLib21.y);
  float selenaLib35 = ((selenaLib17 + selenaLib34) * 0.5);
  float selenaLib36 = selenaLib35;
  float selenaLib37 = progress;
  float selenaLib38 = ((clamp(selenaLib36, 0.0, 1.0) - (clamp(selenaLib37, 0.0, 1.0) * 1.02)) + 0.01);
  float mask = selenaLib38;
  if ((mask < 0.0)) {
    discard;
  }
  vec3 n = normalize(vWorldNormal);
  vec3 lit = (baseColor * (ambient + (sunColor * max(dot(n, normalize(sunDir)), 0.0))));
  float selenaLib39 = mask;
  float selenaLib40 = edgeWidth;
  float selenaLib41 = (1.0 - smoothstep(0.0, max(selenaLib40, 0.0001), selenaLib39));
  fragColor = vec4((lit + ((edgeColor * selenaLib41) * ((progress > 0.0) ? 1.0 : 0.0))), 1.0);
}
