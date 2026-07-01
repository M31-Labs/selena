#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float light_ambient;
uniform vec3 light_dir;
uniform float cutoff;
in vec3 vWorldNormal;
out vec4 fragColor;

void main() {
  vec3 n = normalize(vWorldNormal);
  float lambert = max(dot(n, (-light_dir)), (-cutoff));
  fragColor = vec4((baseColor * (light_ambient + lambert)), 1.0);
}
