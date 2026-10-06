precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float period;
varying vec2 vUv;

void main() {
  float angular = mod(((-1.25) + vUv.x), period);
  vec2 vector = mod((vec2((-3.25), 5.25) + vUv), period);
  gl_FragColor = vec4(vec3(angular, vector.x, vector.y), 1.0);
}
