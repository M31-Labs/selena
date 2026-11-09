precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 deepColor;
uniform vec3 skyColor;
uniform float waveScale;
uniform float choppy;
uniform vec3 cameraPos;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform float time;
varying vec3 vWorldPos;

void main() {
  vec2 selenaLib0 = vWorldPos.xz;
  float selenaLib1 = time;
  float selenaLib2 = waveScale;
  float selenaLib3 = choppy;
  vec2 selenaLib4 = (selenaLib0 * selenaLib2);
  vec2 selenaLib5 = (vec2(0.8, 0.6) * cos((dot(selenaLib4, vec2(0.8, 0.6)) + (selenaLib1 * 1.3))));
  vec2 selenaLib6 = (vec2((-0.4), 0.9) * (cos(((dot(selenaLib4, vec2((-0.4), 0.9)) * 1.7) + (selenaLib1 * 1.9))) * 0.6));
  vec2 selenaLib7 = (vec2(0.95, (-0.3)) * (cos(((dot(selenaLib4, vec2(0.95, (-0.3))) * 2.9) + (selenaLib1 * 2.7))) * 0.35));
  vec2 selenaLib8 = (((selenaLib5 + selenaLib6) + selenaLib7) * selenaLib3);
  vec3 selenaLib9 = normalize(vec3((-selenaLib8.x), 1.0, (-selenaLib8.y)));
  vec3 n = selenaLib9;
  vec3 v = normalize((cameraPos - vWorldPos));
  vec3 l = normalize(sunDir);
  vec3 selenaLib10 = n;
  vec3 selenaLib11 = v;
  float selenaLib12 = (1.0 - clamp(dot(selenaLib10, selenaLib11), 0.0, 1.0));
  float selenaLib13 = (selenaLib12 * selenaLib12);
  float selenaLib14 = (0.02 + (((0.98 * selenaLib13) * selenaLib13) * selenaLib12));
  float f = selenaLib14;
  vec3 glint = ((pow(max(dot(reflect((-l), n), v), 0.0), 240.0) * sunColor) * 6.0);
  gl_FragColor = vec4((mix(deepColor, skyColor, f) + glint), 1.0);
}
