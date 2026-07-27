#version 300 es

out vec2 vUV;

void main() {
  const vec2[3] positions = vec2[3](
    vec2(-1.0, -1.0), vec2(3.0, -1.0), vec2(-1.0, 3.0)
  );
  vUV = positions[gl_VertexID] * 0.5 + 0.5;
  gl_Position = vec4(positions[gl_VertexID], 0.0, 1.0);
}
