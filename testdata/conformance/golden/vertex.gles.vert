#version 300 es
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float gridSize;
uniform float amplitude;
out vec3 worldPos;

void main() {
  uint vertexIndex = uint(gl_VertexID);
  float fi = float(vertexIndex);
  float col = fract((fi / gridSize));
  float row = (floor((fi / gridSize)) / gridSize);
  float x = ((col * 2.0) - 1.0);
  float z = ((row * 2.0) - 1.0);
  float y = (sin((x * 6.2831855)) * amplitude);
  vec3 p = vec3(x, y, z);
  worldPos = p;
  gl_Position = (mvp * vec4(x, y, z, 1.0));
}
