#version 300 es
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float gridSize;
out float shade;

void main() {
  uint vertexIndex = uint(gl_VertexID);
  float fi = 0.0;
  if ((gridSize > 0.0)) {
    fi = float(vertexIndex);
  }
  float col = fract((fi / gridSize));
  float row = (floor((fi / gridSize)) / gridSize);
  shade = col;
  gl_Position = (mvp * vec4(((col * 2.0) - 1.0), 0.0, ((row * 2.0) - 1.0), 1.0));
}
