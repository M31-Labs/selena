precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec4 labelRect;
uniform vec4 numberRect;
uniform vec3 shirtColor;
uniform vec3 numberColor;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
varying vec2 vUv;
varying vec3 vWorldNormal;
uniform sampler2D labels;

void main() {
  vec2 selenaLib0 = vUv;
  vec4 selenaLib1 = numberRect;
  vec2 selenaLib2 = ((selenaLib0 - selenaLib1.xy) / max((selenaLib1.zw - selenaLib1.xy), vec2(0.000001, 0.000001)));
  vec2 selenaLib3 = selenaLib2;
  vec4 selenaLib4 = labelRect;
  vec2 selenaLib5 = mix(selenaLib4.xy, selenaLib4.zw, clamp(selenaLib3, vec2(0.0, 0.0), vec2(1.0, 1.0)));
  vec4 glyph = texture2D(labels, selenaLib5);
  vec2 selenaLib6 = vUv;
  vec4 selenaLib7 = numberRect;
  vec2 selenaLib8 = selenaLib6;
  vec4 selenaLib9 = selenaLib7;
  vec2 selenaLib10 = ((selenaLib8 - selenaLib9.xy) / max((selenaLib9.zw - selenaLib9.xy), vec2(0.000001, 0.000001)));
  vec2 selenaLib11 = selenaLib10;
  float selenaLib12 = (((step(0.0, selenaLib11.x) * step(selenaLib11.x, 1.0)) * step(0.0, selenaLib11.y)) * step(selenaLib11.y, 1.0));
  float ink = (glyph.a * selenaLib12);
  vec3 n = normalize(vWorldNormal);
  vec3 light = (ambient + (sunColor * max(dot(n, normalize(sunDir)), 0.0)));
  gl_FragColor = vec4((mix(shirtColor, (numberColor * glyph.rgb), ink) * light), 1.0);
}
