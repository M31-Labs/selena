precision mediump float;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float depthFade;
varying vec2 vUv;
uniform sampler2D bump;
uniform sampler2D planarReflection;
uniform sampler2D opaqueColor;
uniform highp sampler2D opaqueDepth;

void main() {
  float b = texture2D(bump, vUv).r;
  vec3 refl = texture2D(planarReflection, (vUv + vec2((b * 0.02), 0.0))).rgb;
  vec3 under = texture2D(opaqueColor, vUv).rgb;
  float depth = texture2D(opaqueDepth, vUv).x;
  float k = clamp((depth * depthFade), 0.0, 1.0);
  gl_FragColor = vec4(mix(under, refl, k), 1.0);
}
