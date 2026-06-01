#version 300 es
precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float light_ambient;
uniform vec3 light_dir;
uniform vec3 tint;
in vec3 vWorldNormal;
out vec4 fragColor;

void main() {
  fragColor = vec4(((baseColor * (light_ambient + max(dot(normalize(vWorldNormal), light_dir), 0.0))) * tint), 1.0);
}
