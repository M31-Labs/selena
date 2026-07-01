precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform highp sampler2D stateTex;
varying vec2 vUv;

void main() {
  vec4 s = texture2D(stateTex, vUv);
  gl_FragColor = vec4(vec3(((s.x * 0.5) + 0.5), ((s.y * 0.5) + 0.5), ((s.z * 0.5) + 0.5)), 1.0);
}
