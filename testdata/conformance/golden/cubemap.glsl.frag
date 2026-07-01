precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
varying vec3 vPosition;
varying vec3 vWorldNormal;
uniform samplerCube sky;

void main() {
  vec3 dir = reflect((-vPosition), normalize(vWorldNormal));
  gl_FragColor = vec4(textureCube(sky, dir).rgb, 1.0);
}
