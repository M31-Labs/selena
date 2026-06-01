precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float light_ambient;
uniform vec3 light_dir;
varying vec2 vUv;
varying vec3 vWorldNormal;
uniform sampler2D albedo;

void main() {
  vec3 c = texture2D(albedo, vUv).rgb;
  gl_FragColor = vec4((c * (light_ambient + max(dot(normalize(vWorldNormal), light_dir), 0.0))), 1.0);
}
