#version 300 es
in vec4 rect;
in vec4 tone;
in float spin;
uniform mat4 mvp;
uniform mat3 normalMatrix;
out vec4 vTone;

void main() {
  uint vertexIndex = uint(gl_VertexID);
  float fi = float(vertexIndex);
  vec2 corner = vec2((fract((fi * 0.5)) * 2.0), floor((fi * 0.5)));
  float c = cos(spin);
  float s = sin(spin);
  vec2 centered = (corner - vec2(0.5, 0.5));
  vec2 turned = vec2(((centered.x * c) - (centered.y * s)), ((centered.x * s) + (centered.y * c)));
  vec2 p = (rect.xy + ((turned + vec2(0.5, 0.5)) * rect.zw));
  vTone = tone;
  gl_Position = (mvp * vec4(p, 0.0, 1.0));
}
