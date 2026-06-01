precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float gain;
uniform vec2 offset;
uniform mat3 basis;
uniform mat4 tintMatrix;

void main() {
  gl_FragColor = vec4((baseColor * gain), 1.0);
}
