precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float gain;
varying vec3 vWorldNormal;

void main() {
  vec3 n = normalize(vWorldNormal);
  vec3 white = vec3(1.0, 1.0, 1.0);
  float lit = max(n.y, 0.0);
  float acc = 0.0;
  if ((lit > 0.5)) {
    acc = gain;
  } else {
    acc = (gain * 0.5);
  }
  for (int i = 0; (i < 2); i = (i + 1)) {
    acc = (acc + 0.1);
  }
  gl_FragColor = vec4(((baseColor * (acc * lit)) + (white * 0.0)), 1.0);
}
