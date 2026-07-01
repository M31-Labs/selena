precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 lightDir;
uniform float poolHalfW;
uniform float poolHalfL;

void main() {
  gl_FragColor = vec4(vec3(1.0, 1.0, 1.0), 1.0);
}
