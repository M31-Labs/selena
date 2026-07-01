#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform vec3 baseColor;
uniform float light_ambient;
uniform vec3 light_dir;
uniform float phase;
in vec2 vUv;
in vec3 vWorldNormal;
out vec4 fragColor;

void main() {
  vec3 n = normalize(vWorldNormal);
  float diffuse = max(dot(n, light_dir), 0.0);
  float ripple = (sin(((vUv.x * 12.0) + phase)) * cos((vUv.y * 12.0)));
  float bands = step(0.5, fract((vUv.y * 6.0)));
  float glow = smoothstep(0.1, 0.9, fract((vUv.x * 4.0)));
  vec3 tangent = cross(n, light_dir);
  float rim = pow((1.0 - diffuse), 3.0);
  vec3 bounce = reflect(light_dir, n);
  float shade = (((((light_ambient + diffuse) + (ripple * 0.15)) + (bands * 0.1)) + (glow * 0.2)) + (rim * 0.25));
  fragColor = vec4((((baseColor * shade) + (sqrt(abs(tangent.x)) * 0.05)) + (bounce.y * 0.02)), 1.0);
}
