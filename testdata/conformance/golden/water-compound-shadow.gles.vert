#version 300 es

out vec2 v_uv;

void main() {
  const vec2[3] positions = vec2[3](
    vec2(-1.0, -1.0), vec2(3.0, -1.0), vec2(-1.0, 3.0)
  );
  const vec2[3] uvs = vec2[3](
    vec2(0.0, 1.0), vec2(2.0, 1.0), vec2(0.0, -1.0)
  );
  v_uv = uvs[gl_VertexID];
  gl_Position = vec4(positions[gl_VertexID], 0.0, 1.0);
}
