#version 300 es
in vec4 rect;
in vec4 tone;
in int region;
uniform mat4 mvp;
uniform mat3 normalMatrix;
out vec4 vTone;
flat out int vRegion;

void main() {
  uint vertexIndex = uint(gl_VertexID);
  uint instanceIndex = uint(gl_InstanceID);
  vec2 corner = vec2(float((vertexIndex & 1u)), float((vertexIndex >> 1u)));
  vec2 p = (rect.xy + (corner * rect.zw));
  float stripe = (1.0 - (0.04 * float((instanceIndex & 3u))));
  vTone = vec4((tone.rgb * stripe), tone.a);
  vRegion = region;
  gl_Position = (mvp * vec4(p, 0.0, 1.0));
}
