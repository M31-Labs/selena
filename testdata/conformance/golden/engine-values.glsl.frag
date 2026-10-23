precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 cameraPos;
uniform vec3 sunDir;
uniform vec3 sunColor;
uniform vec3 ambient;
uniform vec4 fog;
uniform float exposure;
uniform vec3 tint;
varying vec3 vPosition;
varying vec3 vWorldNormal;

void main() {
  vec3 n = normalize(vWorldNormal);
  vec3 v = normalize((cameraPos - vPosition));
  float diffuse = max(dot(n, sunDir), 0.0);
  vec3 lit = (((ambient + (sunColor * diffuse)) * tint) * exposure);
  float dist = length((cameraPos - vPosition));
  float f = (1.0 - exp(((-fog.w) * dist)));
  gl_FragColor = vec4((mix(lit, fog.xyz, f) * (0.5 + (0.5 * dot(n, v)))), 1.0);
}
