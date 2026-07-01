#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float light_ambient;
uniform vec3 light_dir;
in vec3 vWorldNormal;
out vec4 fragColor;

void main() {
  vec3 n = normalize(vWorldNormal);
  fragColor = vec4((baseColor * (light_ambient + max(dot(n, light_dir), 0.0))), 1.0);
}
