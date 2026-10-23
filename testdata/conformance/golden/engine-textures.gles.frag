#version 300 es
precision highp float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float depthFade;
in vec2 vUv;
uniform sampler2D bump;
uniform sampler2D planarReflection;
uniform sampler2D opaqueColor;
uniform highp sampler2D opaqueDepth;
out vec4 fragColor;

void main() {
  float b = texture(bump, vUv).r;
  vec3 refl = texture(planarReflection, (vUv + vec2((b * 0.02), 0.0))).rgb;
  vec3 under = texture(opaqueColor, vUv).rgb;
  float depth = textureLod(opaqueDepth, vUv, 0.0).x;
  float k = clamp((depth * depthFade), 0.0, 1.0);
  fragColor = vec4(mix(under, refl, k), 1.0);
}
