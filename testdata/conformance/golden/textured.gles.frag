#version 300 es
precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float light_ambient;
uniform vec3 light_dir;
in vec2 vUv;
in vec3 vWorldNormal;
uniform sampler2D albedo;
out vec4 fragColor;

void main() {
  vec3 c = texture(albedo, vUv).rgb;
  fragColor = vec4((c * (light_ambient + max(dot(normalize(vWorldNormal), light_dir), 0.0))), 1.0);
}
