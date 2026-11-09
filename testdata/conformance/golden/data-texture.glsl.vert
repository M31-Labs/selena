attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;
uniform mat4 mvp;
uniform mat3 normalMatrix;
uniform float head;
uniform highp sampler2D gains;
uniform highp vec2 _texSize_gains;
varying float vLevel;
varying vec2 vUv;

void main() {
  float columns = _texSize_gains.x;
  int column = int(clamp((uv.x * columns), 0.0, (columns - 1.0)));
  float level = texture2D(gains, (vec2(column, 0) + 0.5) / _texSize_gains).r;
  vLevel = level;
  vUv = uv;
  gl_Position = (mvp * vec4((position + (normal * level)), 1.0));
}
