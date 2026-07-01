attribute float a_vertexIndex;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float poolWidth;
uniform float poolLength;
uniform float poolHeight;
uniform float normalScale;
uniform float opticsEnable;
uniform float resolution;
uniform float time;
uniform float objectKind;
uniform float objectCount;
uniform vec3 lightDir;
uniform vec3 objectCenter;
uniform vec4 objectHalfRadius;
uniform vec4 spheres[32];
uniform highp sampler2D stateTex;
varying vec2 vUv;

void main() {
  float vertexIndex = a_vertexIndex;
  float fi = float(vertexIndex);
  float ox = ((((fi == 1.0) || (fi == 2.0)) || (fi == 4.0)) ? 1.0 : 0.0);
  float oy = ((((fi == 2.0) || (fi == 4.0)) || (fi == 5.0)) ? 1.0 : 0.0);
  vUv = vec2(ox, oy);
  gl_Position = vec4(((ox * 2.0) - 1.0), ((oy * 2.0) - 1.0), 0.0, 1.0);
}
